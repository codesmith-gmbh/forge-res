package main

import (
	"context"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	awsssm "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/codesmith-gmbh/forge/aws/common"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"strings"
)

type proc struct {
	ssm *awsssm.SSM
}

func main() {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		panic(err)
	}
	p := &proc{ssm: awsssm.New(cfg)}
	lambda.Start(cfn.LambdaWrap(p.processEvent))
}

type Properties struct {
	SequenceName, Expression string
}

func properties(input map[string]interface{}) (Properties, error) {
	var properties Properties
	if err := mapstructure.Decode(input, &properties); err != nil {
		return properties, err
	}
	if !strings.HasPrefix(properties.SequenceName, "/") {
		return properties, errors.Errorf("name %s must start with an /", properties.SequenceName)
	}
	if properties.Expression == "" {
		properties.Expression = "x"
	}
	if _, err := common.Eval(properties.Expression, 1); err != nil {
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
		if common.IsForgeSsmParameterARN(event.PhysicalResourceID) {
			_, err := p.ssm.DeleteParameterRequest(&awsssm.DeleteParameterInput{
				Name: &event.PhysicalResourceID,
			}).Send(ctx)
			if err != nil {
				return event.PhysicalResourceID, nil, errors.Wrapf(err, "could not delete the Sequence %s", properties.SequenceName)
			}
		}
		return event.PhysicalResourceID, nil, nil
	case cfn.RequestCreate:
		return p.putSequence(ctx, properties)
	case cfn.RequestUpdate:
		return p.putSequence(ctx, properties)
	default:
		return "", nil, errors.Errorf("unknown request type %s", event.RequestType)
	}
}

func (p *proc) putSequence(ctx context.Context, properties Properties) (string, map[string]interface{}, error) {
	overwrite := true
	parameterName := common.SequenceParameterName(properties.SequenceName)
	_, err := p.ssm.PutParameterRequest(&awsssm.PutParameterInput{
		Name:      &parameterName,
		Type:      awsssm.ParameterTypeString,
		Value:     &properties.Expression,
		Overwrite: &overwrite,
	}).Send(ctx)
	if err != nil {
		return "", nil, errors.Wrapf(err, "could not put the parameter %s", parameterName)
	}
	return properties.SequenceName, nil, nil
}
