package main

import (
	"context"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	awsssm "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/codesmith-gmbh/cgc/cgccf"
	"github.com/codesmith-gmbh/forge/aws/common"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"strconv"
)

func main() {
	p := newProc()
	cgccf.StartEventProcessor(p)
}

type proc struct {
	ssm *awsssm.Client
}

func newProc() cgccf.EventProcessor {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		return &cgccf.ConstantErrorEventProcessor{Error: err}
	}
	return newProcFromConfig(cfg)
}

func newProcFromConfig(cfg aws.Config) *proc {
	return &proc{ssm: awsssm.New(cfg)}
}

type Properties struct {
	SequenceName string
}

func properties(input map[string]interface{}) (Properties, error) {
	var properties Properties
	if err := mapstructure.Decode(input, &properties); err != nil {
		return properties, err
	}
	if properties.SequenceName == "" {
		return properties, errors.New("Sequence name is required")
	}
	return properties, nil
}

func (p *proc) ProcessEvent(ctx context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	properties, err := properties(event.ResourceProperties)
	if err != nil {
		return "", nil, err
	}
	switch event.RequestType {
	case cfn.RequestDelete:
		return event.PhysicalResourceID, nil, nil
	case cfn.RequestCreate:
		return p.nextValue(ctx, event, properties)
	case cfn.RequestUpdate:
		return p.nextValue(ctx, event, properties)
	default:
		return "", nil, errors.Errorf("unknown request type %s", event.RequestType)
	}
}

func (p *proc) nextValue(ctx context.Context, event cfn.Event, properties Properties) (string, map[string]interface{}, error) {
	physicalId := physicalId(event, properties)
	overwrite := true
	pname := common.SequenceParameterName(properties.SequenceName)
	param, err := p.ssm.GetParameterRequest(&awsssm.GetParameterInput{
		Name: &pname,
	}).Send(ctx)
	if err != nil {
		return physicalId, nil, errors.Wrapf(err, "unable to get the parameter %s", pname)
	}
	expression := *param.Parameter.Value
	next, err := p.ssm.PutParameterRequest(&awsssm.PutParameterInput{
		Name:      &pname,
		Value:     &expression,
		Type:      awsssm.ParameterTypeString,
		Overwrite: &overwrite,
	}).Send(ctx)
	if err != nil {
		return physicalId, nil, errors.Wrapf(err, "unable to put the parameter %s", pname)
	}
	// The initial version is 1 (when the Sequence is created, it means that the first real value will be 2. As we
	// want to start with 1, we decrement the value obtain from incrementing the parameter.
	value, err := common.Eval(expression, *next.Version-1)
	if err != nil {
		return physicalId, nil, err
	}
	valueText := strconv.FormatInt(value, 10)
	data := make(map[string]interface{}, 1)
	data["ValueText"] = valueText
	data["Value"] = value
	return physicalId, data, nil
}

func physicalId(event cfn.Event, properties Properties) string {
	return event.LogicalResourceID + "-" + properties.SequenceName
}
