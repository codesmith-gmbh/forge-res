package main

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/codesmith-gmbh/cgc/cgccf"
	"github.com/codesmith-gmbh/forge/aws/common"

	"github.com/aws/aws-lambda-go/cfn"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

func main() {
	p := newProc()
	cgccf.StartEventProcessor(p)
}

type proc struct {
	rds *rds.Client
}

func newProc() cgccf.EventProcessor {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		return &cgccf.ConstantErrorEventProcessor{Error: err}
	}
	return newProcFromConfig(cfg)
}

func newProcFromConfig(cfg aws.Config) *proc {
	return &proc{rds: rds.New(cfg)}
}

type Properties struct {
	DbInstanceIdentifier string
}

func properties(input map[string]interface{}) (Properties, error) {
	var properties Properties
	if err := mapstructure.Decode(input, &properties); err != nil {
		return properties, err
	}
	if properties.DbInstanceIdentifier == "" {
		return properties, errors.New("db instance identifier must be defined")
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
		fallthrough
	case cfn.RequestUpdate:
		resourceId, err := p.resourceId(ctx, event, properties)
		return resourceId, nil, err
	default:
		return common.UnknownRequestType(event)
	}
}

func (p *proc) resourceId(ctx context.Context, event cfn.Event, properties Properties) (string, error) {
	dbs, err := p.rds.DescribeDBInstancesRequest(&rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: &properties.DbInstanceIdentifier,
	}).Send(ctx)
	if err != nil {
		return "", errors.Wrapf(err, "could not describe the instance %s", properties.DbInstanceIdentifier)
	}
	return *dbs.DBInstances[0].DbiResourceId, nil
}
