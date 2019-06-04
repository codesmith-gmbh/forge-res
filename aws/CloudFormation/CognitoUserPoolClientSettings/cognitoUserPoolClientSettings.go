// # hello
package main

import (
	"context"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/codesmith-gmbh/forge/aws/common"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"strconv"
)

// TODO@stan: add analytics.
type Properties struct {
	AllowedOAuthFlows               []cip.OAuthFlowType
	AllowedOAuthFlowsUserPoolClient string
	AllowedOAuthScopes              []string
	CallbackURLs                    []string
	LogoutURLs                      []string
	SupportedIdentityProviders      []string
	UserPoolId                      string
	UserPoolClientId                string

	allowedOAuthFlowsUserPoolClient bool
}

func validateProperties(input map[string]interface{}) (Properties, error) {
	var properties Properties
	if err := mapstructure.Decode(input, &properties); err != nil {
		return properties, err
	}
	if properties.UserPoolId == "" {
		return properties, errors.New("UserPoolId is obligatory")
	}
	if properties.UserPoolClientId == "" {
		return properties, errors.New("UserClientPoolId is obligatory")
	}
	allowedOAuthFlowsUserPoolClient, err := strconv.ParseBool(properties.AllowedOAuthFlowsUserPoolClient)
	if err != nil {
		return properties, errors.Wrapf(err, "AllowedOAuthFlowsUserPoolClient not a boolean, %+v", properties)
	}
	properties.allowedOAuthFlowsUserPoolClient = allowedOAuthFlowsUserPoolClient
	return properties, nil
}

func main() {
	cfg := common.MustConfig()
	p := newProc(cfg)
	lambda.Start(cfn.LambdaWrap(p.processEvent))
}

type proc struct {
	cog *cip.CognitoIdentityProvider
}

func newProc(cfg aws.Config) *proc {
	return &proc{cog: cip.New(cfg)}
}

func (p *proc) processEvent(ctx context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	properties, err := validateProperties(event.ResourceProperties)
	if err != nil {
		return event.PhysicalResourceID, nil, err
	}
	switch event.RequestType {
	case cfn.RequestDelete:
		return p.deleteCognitoUserPoolClientSettings(ctx, event, properties)
	case cfn.RequestCreate, cfn.RequestUpdate:
		return p.createCognitoUserPoolClientSettings(ctx, event, properties)
	default:
		return event.PhysicalResourceID, nil, errors.Errorf("unknown request type %s", event.RequestType)
	}
}

func (p *proc) createCognitoUserPoolClientSettings(ctx context.Context, event cfn.Event, properties Properties) (string, map[string]interface{}, error) {
	if err := p.updateClient(ctx, properties); err != nil {
		return event.PhysicalResourceID, nil, err
	}
	return properties.UserPoolClientId, nil, nil
}

func (p *proc) deleteCognitoUserPoolClientSettings(ctx context.Context, event cfn.Event, properties Properties) (string, map[string]interface{}, error) {
	properties.allowedOAuthFlowsUserPoolClient = false
	properties.AllowedOAuthFlows = []cip.OAuthFlowType{}
	properties.AllowedOAuthScopes = []string{}
	properties.CallbackURLs = []string{}
	properties.LogoutURLs = []string{}
	properties.SupportedIdentityProviders = []string{}
	err := p.updateClient(ctx, properties)
	if err != nil {
		switch err := errors.Cause(err).(type) {
		case awserr.RequestFailure:
			if err.StatusCode() == 400 && err.Code() == "ResourceNotFoundException" {
				return event.PhysicalResourceID, nil, nil
			}
		}
		return event.PhysicalResourceID, nil, err
	}
	return event.PhysicalResourceID, nil, nil
}

func (p *proc) updateClient(ctx context.Context, properties Properties) error {
	var defaultRedirectURI *string
	if len(properties.CallbackURLs) > 0 {
		defaultRedirectURI = &properties.CallbackURLs[0]
	}
	_, err := p.cog.UpdateUserPoolClientRequest(&cip.UpdateUserPoolClientInput{
		AllowedOAuthFlows:               properties.AllowedOAuthFlows,
		AllowedOAuthFlowsUserPoolClient: &properties.allowedOAuthFlowsUserPoolClient,
		AllowedOAuthScopes:              properties.AllowedOAuthScopes,
		DefaultRedirectURI:              defaultRedirectURI,
		CallbackURLs:                    properties.CallbackURLs,
		ClientId:                        &properties.UserPoolClientId,
		LogoutURLs:                      properties.LogoutURLs,
		SupportedIdentityProviders:      properties.SupportedIdentityProviders,
		UserPoolId:                      &properties.UserPoolId,
	}).Send(ctx)
	if err != nil {
		return errors.Wrapf(err, "could not update the user pool client %s", properties.UserPoolClientId)
	}
	return nil
}
