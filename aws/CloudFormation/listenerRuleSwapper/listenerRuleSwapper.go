// # ListenerRuleSwapper
//
// The `listenerRuleSwapper` custom resource is meant for a very specific use case of an application served by an ALB
// and that requires interruption on release.
//
// ## Syntax
//
// To create an listenerRuleSwapper resource, add the following resource to your cloudformation
// template (yaml notation, json is similar)
//
// ```yaml
// ListenerRuleSwapper:
//   Type: Custom::ListenerRuleSwapper
//   Properties:
//     ServiceToken: !ImportValue HyperdriveCore-ListenerRuleSwapper
//	   ListenerArn: <listener arn>
//     Rule1Arn: <rule arn>
//	   Rule2Arn: <rule arn>
//     Trigger: <changing value>
// ```
package main

import (
	"context"
	"github.com/DEEP-IMPACT-AG/hyperdrive/common"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/elbv2"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"strconv"
)

var elb *elbv2.ELBV2

// The lambda is started using the AWS lambda go sdk. The handler function
// does the actual work of creating the log group. Cloudformation sends an
// event to signify that a resources must be created, updated or deleted.
func main() {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		panic(err)
	}
	elb = elbv2.New(cfg)
	lambda.Start(cfn.LambdaWrap(processEvent))
}

// The Properties is the main data structure for the listenerRuleSwapper resource and
// is defined as a go struct. The struct mirrors the properties as defined above.
// We use the library [mapstructure](https://github.com/mitchellh/mapstructure) to
// decode the generic map from the cloudformation event to the struct.
type Properties struct {
	ListenerArn string
	Rule1Arn    string
	Rule2Arn    string
}

func properties(input map[string]interface{}) (Properties, error) {
	var properties Properties
	if err := mapstructure.Decode(input, &properties); err != nil {
		return properties, err
	}
	return properties, nil
}

// To process an event, we first decode the resource properties and analyse
// the event. We have 2 cases.
//
// 1. Delete: The delete case it self has 3 sub cases:
//    1. the physical resource id is a failure id, then this is a NOP;
//    2. the stack is being deleted: in that case, we delete all the images in the
//       repository.
//    3. the stack is not being delete: it is a NOP as well.
// 2. Create, Update: In that case, it is a NOP, the physical ID is simply
//    the logical ID of the resource.
func processEvent(ctx context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	properties, err := properties(event.ResourceProperties)
	if err != nil {
		return "", nil, err
	}
	switch event.RequestType {
	case cfn.RequestDelete:
		if !common.IsFailurePhysicalResourceId(event.PhysicalResourceID) {
			return swapRules(event.LogicalResourceID, properties)
		}
		return event.LogicalResourceID, nil, nil
	case cfn.RequestCreate:
		return swapRules(event.LogicalResourceID, properties)
	case cfn.RequestUpdate:
		return swapRules(event.LogicalResourceID, properties)
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

func swapRules(resId string, prop Properties) (string, map[string]interface{}, error) {
	rules, err := elb.DescribeRulesRequest(&elbv2.DescribeRulesInput{
		ListenerArn: &prop.ListenerArn,
		RuleArns:    []string{prop.Rule1Arn, prop.Rule2Arn},
	}).Send()
	var rule1Priority *int64
	var rule2Priority *int64
	for _, rule := range rules.Rules {
		if *rule.RuleArn == prop.Rule1Arn {
			rule1Priority = rulePriority(rule)
		}
	}
	if err != nil {
		return resId, nil, errors.Wrapf(err, "could not fetch the rules %s and %s on the listener %s", prop.Rule1Arn, prop.Rule2Arn, prop.ListenerArn)
	}
	_, err = elb.SetRulePrioritiesRequest(&elbv2.SetRulePrioritiesInput{
		RulePriorities: []elbv2.RulePriorityPair{
			{RuleArn: &prop.Rule1Arn, Priority: rule2Priority},
			{RuleArn: &prop.Rule2Arn, Priority: rule1Priority},
		},
	}).Send()
	if err != nil {
		return resId, nil, errors.Wrapf(err, "could not swap the rules %s and %s on the listener %s", prop.Rule1Arn, prop.Rule2Arn, prop.ListenerArn)
	}
	return resId, nil, nil
}
