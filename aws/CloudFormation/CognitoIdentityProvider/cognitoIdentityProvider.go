package main

import (
	"context"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/codesmith-gmbh/cgc/cgccf"
	"github.com/codesmith-gmbh/forge/aws/common"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"strings"
)

type Properties struct {
	UserPoolId, ProviderName, ClientIdParameter, ClientSecretParameter string
	ProviderType                                                       cognitoidentityprovider.IdentityProviderTypeType
	AuthorizeScopes                                                    []string
	AttributeMapping                                                   map[string]string
}

func (p *proc) validateProperties(ctx context.Context, input map[string]interface{}) (Properties, map[string]string, error) {
	var properties Properties
	if err := mapstructure.Decode(input, &properties); err != nil {
		return properties, nil, err
	}
	details := make(map[string]string)
	providerType := properties.ProviderType
	switch providerType {
	case cognitoidentityprovider.IdentityProviderTypeTypeGoogle:
		details["authorize_url"] = "https://accounts.google.com/o/oauth2/v2/auth"
		details["authorize_scopes"] = strings.Join(properties.AuthorizeScopes, " ")
		details["attributes_url_add_attributes"] = "true"
		details["token_url"] = "https://www.googleapis.com/oauth2/v4/token"
		details["attributes_url"] = "https://people.googleapis.com/v1/people/me?personFields="
		details["oidc_issuer"] = "https://accounts.google.com"
		details["token_request_method"] = "POST"
	default:
		return properties, nil, errors.Errorf("unknown provider type %s", providerType)
	}
	clientId, err := p.readParameter(ctx, properties.ClientIdParameter)
	if err != nil {
		return properties, nil, err
	}
	clientSecret, err := p.readParameter(ctx, properties.ClientSecretParameter)
	if err != nil {
		return properties, nil, err
	}
	details["client_id"] = clientId
	details["client_secret"] = clientSecret
	return properties, details, nil
}

func main() {
	p := newProc()
	cgccf.StartEventProcessor(p)
}

type proc struct {
	idp *cognitoidentityprovider.Client
	ssm *ssm.Client
}

func newProc() cgccf.EventProcessor {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		return &cgccf.ConstantErrorEventProcessor{Error: err}
	}
	return newProcFromConfig(cfg)
}

func newProcFromConfig(cfg aws.Config) *proc {
	return &proc{idp: cognitoidentityprovider.New(cfg), ssm: ssm.New(cfg)}
}

func (p *proc) ProcessEvent(ctx context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	properties, providerDetails, err := p.validateProperties(ctx, event.ResourceProperties)
	if err != nil {
		return "", nil, err
	}
	switch event.RequestType {
	case cfn.RequestDelete:
		return p.deleteIdentityProvider(ctx, event, properties)
	case cfn.RequestUpdate:
		return p.updateIdentityProvider(ctx, event, properties, providerDetails)
	case cfn.RequestCreate:
		return p.createIdentityProvider(ctx, event, properties, providerDetails)
	default:
		return common.UnknownRequestType(event)
	}
}

func (p *proc) readParameter(ctx context.Context, parameterName string) (string, error) {
	decrypt := true
	param, err := p.ssm.GetParameterRequest(&ssm.GetParameterInput{
		Name:           &parameterName,
		WithDecryption: &decrypt,
	}).Send(ctx)
	if err != nil {
		return "", errors.Wrapf(err, "could not read parameter %s", parameterName)
	}
	return *param.Parameter.Value, err
}

func (p *proc) createIdentityProvider(ctx context.Context, event cfn.Event, properties Properties, providerDetails map[string]string) (string, map[string]interface{}, error) {
	_, err := p.idp.CreateIdentityProviderRequest(&cognitoidentityprovider.CreateIdentityProviderInput{
		UserPoolId:       &properties.UserPoolId,
		ProviderName:     &properties.ProviderName,
		ProviderType:     properties.ProviderType,
		AttributeMapping: properties.AttributeMapping,
		ProviderDetails:  providerDetails,
	}).Send(ctx)
	if err != nil {
		return "", nil, err
	}
	data := map[string]interface{}{"UserPoolId": properties.UserPoolId, "ProviderName": properties.ProviderName}
	return properties.UserPoolId + "/" + properties.ProviderName, data, nil
}

func physicalResourceID(properties Properties) string {
	return properties.UserPoolId + "/" + properties.ProviderName
}

func (p *proc) updateIdentityProvider(ctx context.Context, event cfn.Event, properties Properties, providerDetails map[string]string) (string, map[string]interface{}, error) {
	oldUserPoolId := event.OldResourceProperties["UserPoolId"].(string)
	oldProviderName := event.OldResourceProperties["ProviderName"].(string)
	if properties.UserPoolId != oldUserPoolId || properties.ProviderName != oldProviderName {
		return p.createIdentityProvider(ctx, event, properties, providerDetails)
	}
	_, err := p.idp.UpdateIdentityProviderRequest(&cognitoidentityprovider.UpdateIdentityProviderInput{
		UserPoolId:       &properties.UserPoolId,
		ProviderName:     &properties.ProviderName,
		AttributeMapping: properties.AttributeMapping,
		ProviderDetails:  providerDetails,
	}).Send(ctx)
	if err != nil {
		return event.PhysicalResourceID, event.ResourceProperties, errors.Wrapf(err, "could not update the identity provider %s for the user pool %s", properties.ProviderName, properties.UserPoolId)
	}
	return event.PhysicalResourceID, event.ResourceProperties, nil
}

func (p *proc) deleteIdentityProvider(ctx context.Context, event cfn.Event, properties Properties) (string, map[string]interface{}, error) {
	_, err := p.idp.DeleteIdentityProviderRequest(&cognitoidentityprovider.DeleteIdentityProviderInput{
		UserPoolId:   &properties.UserPoolId,
		ProviderName: &properties.ProviderName,
	}).Send(ctx)
	if err != nil {
		awsErr, ok := err.(awserr.RequestFailure)
		if !ok || awsErr.StatusCode() != 400 || awsErr.Code() != "ResourceNotFoundException" {
			return event.PhysicalResourceID, nil, errors.Wrapf(err, "could not delete the identity provider %s", event.PhysicalResourceID)
		}
	}
	return event.PhysicalResourceID, nil, nil
}
