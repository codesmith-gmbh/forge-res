// # ListenerRuleSwapper
//
// The `ListenerRuleSwapper` custom resource is meant for a very specific use case of an application served by an ALB
// and that requires interruption on release.
//
// ## Syntax
//
// To create an ListenerRuleSwapper resource, add the following resource to your cloudformation
// template (yaml notation, json is similar)
//
// ```yaml
// ListenerRuleSwapper:
//   Type: Custom::ListenerRuleSwapper
//   Properties:
//     ServiceToken: !ImportValue ForgeResources-ListenerRuleSwapper
//	   ListenerArn: <listener arn>
//     Rule1Arn: <rule arn>
//	   Rule2Arn: <rule arn>
//     Trigger: <changing value>
// ```
package main

import (
	"context"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/elbv2"
	"github.com/codesmith-gmbh/forge/aws/common"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"strconv"
)

var elb *elbv2.ELBV2

func main() {
	cfg := common.MustConfig()
	p := &proc{cf: cloudformation.New(cfg), elb: elbv2.New(cfg)}
	lambda.Start(cfn.LambdaWrap(p.processEvent))
}

type proc struct {
	cf  *cloudformation.CloudFormation
	elb *elbv2.ELBV2
}

type Properties struct {
	ListenerArn string
	Rule1Arn    string
	Rule2Arn    string
	Trigger     string
}

func properties(input map[string]interface{}) (Properties, error) {
	var properties Properties
	if err := mapstructure.Decode(input, &properties); err != nil {
		return properties, err
	}
	return properties, nil
}

func (p *proc) processEvent(ctx context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	properties, err := properties(event.ResourceProperties)
	if err != nil {
		return "", nil, err
	}
	switch event.RequestType {
	case cfn.RequestDelete:
		isBeingReplaced, err := p.isBeingReplaced(ctx, event)
		if err != nil {
			return event.PhysicalResourceID, nil, nil
		}
		if isBeingReplaced {
			return swapRules(ctx, event, properties)
		}
		return event.LogicalResourceID, nil, nil
	case cfn.RequestCreate:
		return event.LogicalResourceID, nil, nil
	case cfn.RequestUpdate:
		return swapRules(ctx, event, properties)
	default:
		return event.LogicalResourceID, nil, errors.Errorf("unknown request type %s", event.RequestType)
	}
}

func rulePriority(rule elbv2.Rule) *int64 {
	prio, err := strconv.ParseInt(*rule.Priority, 10, 64)
	if err != nil {
		panic(err)
	}
	return &prio
}

func swapRules(ctx context.Context, event cfn.Event, prop Properties) (string, map[string]interface{}, error) {
	rules, err := elb.DescribeRulesRequest(&elbv2.DescribeRulesInput{
		ListenerArn: &prop.ListenerArn,
		RuleArns:    []string{prop.Rule1Arn, prop.Rule2Arn},
	}).Send(ctx)
	if err != nil {
		return physicalResourceID(event, prop), nil, errors.Wrapf(err, "could not fetch the rules %s and %s on the listener %s", prop.Rule1Arn, prop.Rule2Arn, prop.ListenerArn)
	}
	var rule1Priority *int64
	var rule2Priority *int64
	for _, rule := range rules.Rules {
		if *rule.RuleArn == prop.Rule1Arn {
			rule1Priority = rulePriority(rule)
		}
	}
	_, err = elb.SetRulePrioritiesRequest(&elbv2.SetRulePrioritiesInput{
		RulePriorities: []elbv2.RulePriorityPair{
			{RuleArn: &prop.Rule1Arn, Priority: rule2Priority},
			{RuleArn: &prop.Rule2Arn, Priority: rule1Priority},
		},
	}).Send(ctx)
	if err != nil {
		return physicalResourceID(event, prop), nil, errors.Wrapf(err, "could not swap the rules %s and %s on the listener %s", prop.Rule1Arn, prop.Rule2Arn, prop.ListenerArn)
	}
	return physicalResourceID(event, prop), nil, nil
}

func physicalResourceID(event cfn.Event, properties Properties) string {
	return event.LogicalResourceID + "-" + properties.Trigger
}

// If the physical id of a resource being deleted is different from the physical id of the resource with the same
// logical id in the stack, then we have a replacement; otherwise, we have a simple deletion.
func (p *proc) isBeingReplaced(ctx context.Context, event cfn.Event) (bool, error) {
	res, err := p.cf.DescribeStackResourceRequest(&cloudformation.DescribeStackResourceInput{
		StackName:         &event.StackID,
		LogicalResourceId: &event.LogicalResourceID,
	}).Send(ctx)
	if err != nil {
		return false, errors.Wrapf(err, "could not describe the resource %s on the stack %s", event.StackID, event.LogicalResourceID)
	}
	return *res.StackResourceDetail.PhysicalResourceId != event.PhysicalResourceID, nil
}
