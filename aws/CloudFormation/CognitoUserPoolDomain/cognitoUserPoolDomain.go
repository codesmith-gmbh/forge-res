package main

import (
	"context"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/codesmith-gmbh/forge/aws/common"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"strings"
)

func main() {
	cfg := common.MustConfig()
	p := newProc(cfg)
	lambda.Start(cfn.LambdaWrap(p.processEvent))
}

type proc struct {
	idp *cognitoidentityprovider.CognitoIdentityProvider
}

func newProc(cfg aws.Config) *proc {
	return &proc{idp: cognitoidentityprovider.New(cfg)}
}

type Properties struct {
	Domain             string
	UserPoolId         string
	CustomDomainConfig cognitoidentityprovider.CustomDomainConfigType
}

func validateProperties(input map[string]interface{}) (Properties, error) {
	var properties Properties
	if err := mapstructure.Decode(input, &properties); err != nil {
		return properties, err
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
		return p.deleteDomain(event, properties)
	case cfn.RequestCreate, cfn.RequestUpdate:
		return p.createDomain(event, properties)
	default:
		return common.UnknownRequestType(event)
	}
}

func (p *proc) createDomain(event cfn.Event, properties Properties) (string, map[string]interface{}, error) {
	var out *cognitoidentityprovider.CreateUserPoolDomainOutput
	var err error
	if properties.CustomDomainConfig.CertificateArn == nil {
		out, err = p.idp.CreateUserPoolDomainRequest(&cognitoidentityprovider.CreateUserPoolDomainInput{
			Domain:     &properties.Domain,
			UserPoolId: &properties.UserPoolId,
		}).Send()
	} else {
		out, err = p.idp.CreateUserPoolDomainRequest(&cognitoidentityprovider.CreateUserPoolDomainInput{
			Domain:             &properties.Domain,
			UserPoolId:         &properties.UserPoolId,
			CustomDomainConfig: &properties.CustomDomainConfig,
		}).Send()
	}
	if err != nil {
		return "", nil, errors.Wrap(err, "Could not create the CognitoUserPoolDomain")
	}
	var cloudFrontDomain string
	var domain string
	if out.CloudFrontDomain == nil {
		cloudFrontDomain = ""
		domain = properties.Domain + ".auth." + p.idp.Region + ".amazoncognito.com"
	} else {
		cloudFrontDomain = *out.CloudFrontDomain
		domain = properties.Domain
	}
	return properties.Domain,
		map[string]interface{}{
			"UserPoolId":       properties.UserPoolId,
			"CloudFrontDomain": cloudFrontDomain,
			"Domain":           domain,
		},
		nil
}

func (p *proc) deleteDomain(event cfn.Event, properties Properties) (string, map[string]interface{}, error) {
	_, err := p.idp.DeleteUserPoolDomainRequest(&cognitoidentityprovider.DeleteUserPoolDomainInput{
		Domain:     &event.PhysicalResourceID,
		UserPoolId: &properties.UserPoolId,
	}).Send()
	if err != nil {
		awsErr, ok := err.(awserr.RequestFailure)
		if !ok || awsErr.StatusCode() != 400 || !strings.HasPrefix(awsErr.Message(), "No such domain or user pool exists") {
			return event.PhysicalResourceID, nil, errors.Wrapf(err, "could not delete the CognitoUserPoolDomain %s", event.PhysicalResourceID)
		}
	}
	return event.PhysicalResourceID, nil, nil
}