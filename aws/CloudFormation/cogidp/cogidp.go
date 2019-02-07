package main

import (
	"context"
	"github.com/DEEP-IMPACT-AG/hyperdrive/common"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	awsssm "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"strings"
)

var idp *cognitoidentityprovider.CognitoIdentityProvider
var ssm *awsssm.SSM

// The lambda is started using the AWS lambda go sdk. The handler function
// does the actual work of creating the apikey. Cloudformation sends an
// event to signify that a resources must be created, updated or deleted.
func main() {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		panic(err)
	}
	idp = cognitoidentityprovider.New(cfg)
	ssm = awsssm.New(cfg)
	lambda.Start(cfn.LambdaWrap(processEvent))
}

type ProviderProperties struct {
	UserPoolId, ProviderName, ClientIdParameter, ClientSecretParameter string
	ProviderType                                                       cognitoidentityprovider.IdentityProviderTypeType
	AuthorizeScopes                                                    []string
	AttributeMapping                                                   map[string]string
}

func readParameter(parameterName string) (string, error) {
	decrypt := true
	param, err := ssm.GetParameterRequest(&awsssm.GetParameterInput{
		Name:           &parameterName,
		WithDecryption: &decrypt,
	}).Send()
	if err != nil {
		return "", errors.Wrapf(err, "could not read parameter %s", parameterName)
	}
	return *param.Parameter.Value, err
}

func providerProperties(input map[string]interface{}) (ProviderProperties, map[string]string, error) {
	var properties ProviderProperties
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
	clientId, err := readParameter(properties.ClientIdParameter)
	if err != nil {
		return properties, nil, err
	}
	clientSecret, err := readParameter(properties.ClientSecretParameter)
	if err != nil {
		return properties, nil, err
	}
	details["client_id"] = clientId
	details["client_secret"] = clientSecret
	return properties, details, nil
}

func createIdentityProvider(event cfn.Event, properties ProviderProperties, providerDetails map[string]string) (string, map[string]interface{}, error) {
	_, err := idp.CreateIdentityProviderRequest(&cognitoidentityprovider.CreateIdentityProviderInput{
		UserPoolId:       &properties.UserPoolId,
		ProviderName:     &properties.ProviderName,
		ProviderType:     properties.ProviderType,
		AttributeMapping: properties.AttributeMapping,
		ProviderDetails:  providerDetails,
	}).Send()
	if err != nil {
		return common.FailurePhysicalResourceId(event), nil, err
	}
	data := map[string]interface{}{"UserPoolId": properties.UserPoolId, "ProviderName": properties.ProviderName}
	return properties.UserPoolId + "/" + properties.ProviderName, data, nil
}

func updateIdentityProvider(event cfn.Event, properties ProviderProperties, providerDetails map[string]string) (string, map[string]interface{}, error) {
	_, err := idp.UpdateIdentityProviderRequest(&cognitoidentityprovider.UpdateIdentityProviderInput{
		UserPoolId:       &properties.UserPoolId,
		ProviderName:     &properties.ProviderName,
		AttributeMapping: properties.AttributeMapping,
		ProviderDetails:  providerDetails,
	}).Send()
	if err != nil {
		return event.PhysicalResourceID, event.ResourceProperties, errors.Wrapf(err, "could not update the identity provider %s for the user pool %s", properties.ProviderName, properties.UserPoolId)
	}
	return event.PhysicalResourceID, event.ResourceProperties, nil
}

func processEvent(_ context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	properties, providerDetails, err := providerProperties(event.ResourceProperties)
	if err != nil {
		return "", nil, err
	}
	switch event.RequestType {
	case cfn.RequestDelete:
		if !common.IsFailurePhysicalResourceId(event.PhysicalResourceID) {
			userPoolId := event.ResourceProperties["UserPoolId"].(string)
			providerName := event.ResourceProperties["ProviderName"].(string)
			_, err := idp.DeleteIdentityProviderRequest(&cognitoidentityprovider.DeleteIdentityProviderInput{
				UserPoolId:   &userPoolId,
				ProviderName: &providerName,
			}).Send()
			if err != nil {
				return event.PhysicalResourceID, nil, errors.Wrapf(err, "could not delete the identity provider %s", event.PhysicalResourceID)
			}
		}
		return event.PhysicalResourceID, nil, nil
	case cfn.RequestUpdate:
		oldUserPoolId := event.OldResourceProperties["UserPoolId"].(string)
		oldProviderName := event.OldResourceProperties["ProviderName"].(string)
		if properties.UserPoolId != oldUserPoolId || properties.ProviderName != oldProviderName {
			return createIdentityProvider(event, properties, providerDetails)
		}
		return updateIdentityProvider(event, properties, providerDetails)
	case cfn.RequestCreate:
		return createIdentityProvider(event, properties, providerDetails)
	default:
		return event.PhysicalResourceID, nil, errors.Errorf("unknown request type %s", event.RequestType)
	}
}
