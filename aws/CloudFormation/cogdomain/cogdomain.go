package main

import (
	"context"
	"github.com/DEEP-IMPACT-AG/hyperdrive/common"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

var idp *cognitoidentityprovider.CognitoIdentityProvider

// The lambda is started using the AWS lambda go sdk. The handler function
// does the actual work of creating the apikey. Cloudformation sends an
// event to signify that a resources must be created, updated or deleted.
func main() {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		panic(err)
	}
	idp = cognitoidentityprovider.New(cfg)
	lambda.Start(cfn.LambdaWrap(processEvent))
}

type DomainProperties struct {
	Domain             string
	UserPoolId         string
	CustomDomainConfig cognitoidentityprovider.CustomDomainConfigType
}

func domainProperties(input map[string]interface{}) (DomainProperties, error) {
	var properties DomainProperties
	if err := mapstructure.Decode(input, &properties); err != nil {
		return properties, err
	}
	return properties, nil
}

func createDomain(event cfn.Event, properties DomainProperties) (string, map[string]interface{}, error) {
	var out *cognitoidentityprovider.CreateUserPoolDomainOutput
	var err error
	if properties.CustomDomainConfig.CertificateArn == nil {
		out, err = idp.CreateUserPoolDomainRequest(&cognitoidentityprovider.CreateUserPoolDomainInput{
			Domain:     &properties.Domain,
			UserPoolId: &properties.UserPoolId,
		}).Send()
	} else {
		out, err = idp.CreateUserPoolDomainRequest(&cognitoidentityprovider.CreateUserPoolDomainInput{
			Domain:             &properties.Domain,
			UserPoolId:         &properties.UserPoolId,
			CustomDomainConfig: &properties.CustomDomainConfig,
		}).Send()
	}
	if err != nil {
		return common.FailurePhysicalResourceId(event), nil, errors.Wrap(err, "Could not create the UserPoolDomain")
	}
	var cloudFrontDomain string
	var domain string
	if out.CloudFrontDomain == nil {
		cloudFrontDomain = ""
		domain = properties.Domain + ".auth." + idp.Region + ".amazoncognito.com"
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

func processEvent(ctx context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	properties, err := domainProperties(event.ResourceProperties)
	if err != nil {
		return "", nil, err
	}
	switch event.RequestType {
	case cfn.RequestDelete:
		if !common.IsFailurePhysicalResourceId(event.PhysicalResourceID) {
			userPoolId := event.ResourceProperties["UserPoolId"].(string)
			_, err := idp.DeleteUserPoolDomainRequest(&cognitoidentityprovider.DeleteUserPoolDomainInput{
				Domain:     &event.PhysicalResourceID,
				UserPoolId: &userPoolId,
			}).Send()
			if err != nil {
				return event.PhysicalResourceID, nil, errors.Wrapf(err, "could not delete the UserPoolDomain %s", event.PhysicalResourceID)
			}
		}
		return event.PhysicalResourceID, nil, nil
	case cfn.RequestUpdate:
		return createDomain(event, properties)
	case cfn.RequestCreate:
		return createDomain(event, properties)
	default:
		return event.PhysicalResourceID, nil, errors.Errorf("unknown request type %s", event.RequestType)
	}
}
