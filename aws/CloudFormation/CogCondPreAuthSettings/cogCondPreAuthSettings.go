package main

import (
	"context"
	"encoding/json"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/codesmith-gmbh/cgc/cgcaws"
	"github.com/codesmith-gmbh/forge/aws/common"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"strconv"
)

func main() {
	cfg := cgcaws.MustConfig()
	p := newProc(cfg)
	lambda.Start(cfn.LambdaWrap(p.processEvent))
}

type proc struct {
	ssm *ssm.Client
}

func newProc(cfg aws.Config) *proc {
	return &proc{ssm: ssm.New(cfg)}
}

type Properties struct {
	UserPoolId       string
	UserPoolClientId string
	All              string
	Domains          []string
	Emails           []string
}

func validateProperties(input map[string]interface{}) (Properties, error) {
	var properties Properties
	if err := mapstructure.Decode(input, &properties); err != nil {
		return properties, err
	}
	if properties.UserPoolId == "" {
		return properties, errors.New("UserPoolId is required")
	}
	if properties.UserPoolClientId == "" {
		return properties, errors.New("UserPoolClientId is required")
	}
	return properties, nil
}

func (p *proc) processEvent(ctx context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	properties, err := validateProperties(event.ResourceProperties)
	if err != nil {
		return "", nil, err
	}
	switch event.RequestType {
	case cfn.RequestDelete:
		return p.deleteParameter(ctx, event.PhysicalResourceID)
	case cfn.RequestCreate, cfn.RequestUpdate:
		return p.putParameter(ctx, properties)
	default:
		return common.UnknownRequestType(event)
	}
}

func (p *proc) putParameter(ctx context.Context, properties Properties) (string, map[string]interface{}, error) {
	overwrite := true
	parameterName := common.CogCondPreAuthParameterName(properties.UserPoolId, properties.UserPoolClientId)
	all, err := strconv.ParseBool(properties.All)
	if err != nil {
		return "", nil, errors.Wrapf(err, "All must be a boolean: %s", properties.All)
	}
	data := map[string]interface{}{
		"All":     all,
		"Domains": properties.Domains,
		"Emails":  properties.Emails,
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return "", nil, errors.Wrapf(err, "could not marshal the parameter %s", parameterName)
	}
	dataText := string(dataBytes)
	_, err = p.ssm.PutParameterRequest(&ssm.PutParameterInput{
		Overwrite: &overwrite,
		Name:      &parameterName,
		Type:      ssm.ParameterTypeString,
		Value:     &dataText,
	}).Send(ctx)
	if err != nil {
		return "", nil, errors.Wrapf(err, "could not put the parameter %s", parameterName)
	}
	return parameterName, nil, nil
}

func (p *proc) deleteParameter(ctx context.Context, parameterName string) (string, map[string]interface{}, error) {
	_, err := p.ssm.DeleteParameterRequest(&ssm.DeleteParameterInput{
		Name: &parameterName,
	}).Send(ctx)
	if err != nil {
		awsErr, ok := err.(awserr.RequestFailure)
		if !ok || awsErr.StatusCode() != 400 || awsErr.Code() != "ParameterNotFound" {
			return parameterName, nil, errors.Wrapf(err, "could not delete the parameter %s", parameterName)
		}
	}
	return parameterName, nil, nil
}
