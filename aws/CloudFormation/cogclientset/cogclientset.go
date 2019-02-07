// # hello
package main

import (
	"context"
	common "github.com/DEEP-IMPACT-AG/hyperdrive/common"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"strconv"
)

// The main data structure for the cognito user client setting is defined as a go
// struct. The struct mirrors the properties as defined above. We use the
// library [mapstructure](https://github.com/mitchellh/mapstructure) to
// decode the generic map from the cloudformation event to the struct.
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
}

func properties(input map[string]interface{}) (Properties, error) {
	var properties Properties
	if err := mapstructure.Decode(input, &properties); err != nil {
		return properties, err
	}
	return properties, nil
}

// The lambda is started using the AWS lambda go sdk. The handler function
// does the actual work of creating the certificate. Cloudformation sends
// an event to signify that a resources must be created, updated or
// deleted.
func main() {
	lambda.Start(cfn.LambdaWrap(processEvent))
}

// When processing an event, we first decode the resource properties and
// create a acm client client. We have then 3 cases:
//
// 1. Delete: The delete case it self has 2 sub cases: if the physical
//    resource id is a failure id, then this is a NOP, otherwise we delete
//    the certificate.
// 2. Create: In that case, we proceed to create the certificate,
//    add tags if applicable and collect the DNS CNAME records to construct
//    the attributes of the resource.
// 3. Update: If only the tags have changed, we update them; otherwise, the update
//    requires a replacement and the resource is normally created.
func processEvent(_ context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	cog, err := cogService()
	if err != nil {
		return "", nil, err
	}
	switch event.RequestType {
	case cfn.RequestDelete:
		if !common.IsFailurePhysicalResourceId(event.PhysicalResourceID) {
			updateClient(cog, Properties{})
		}
		return event.PhysicalResourceID, nil, nil
	case cfn.RequestCreate:
		fallthrough
	case cfn.RequestUpdate:
		properties, err := properties(event.ResourceProperties)
		if err != nil {
			return event.PhysicalResourceID, nil, err
		}
		if err = updateClient(cog, properties); err != nil {
			return event.PhysicalResourceID, nil, err
		}
		return properties.UserPoolClientId, nil, nil
	default:
		return event.PhysicalResourceID, nil, errors.Errorf("unknown request type %s", event.RequestType)
	}
}

func updateClient(cog *cip.CognitoIdentityProvider, properties Properties) error {
	allowedOAuthFlowsUserPoolClient, err := strconv.ParseBool(properties.AllowedOAuthFlowsUserPoolClient)
	if err != nil {
		return errors.Wrapf(err, "AllowedOAuthFlowsUserPoolClient not a boolean, %+v", properties)
	}
	_, err = cog.UpdateUserPoolClientRequest(&cip.UpdateUserPoolClientInput{
		AllowedOAuthFlows:               properties.AllowedOAuthFlows,
		AllowedOAuthFlowsUserPoolClient: &allowedOAuthFlowsUserPoolClient,
		AllowedOAuthScopes:              properties.AllowedOAuthScopes,
		DefaultRedirectURI:              &properties.CallbackURLs[0],
		CallbackURLs:                    properties.CallbackURLs,
		ClientId:                        &properties.UserPoolClientId,
		LogoutURLs:                      properties.LogoutURLs,
		SupportedIdentityProviders:      properties.SupportedIdentityProviders,
		UserPoolId:                      &properties.UserPoolId,
	}).Send()
	if err != nil {
		return errors.Wrapf(err, "could not update the user pool client %s", properties.UserPoolClientId)
	}
	return nil
}

// ### SDK client
//
// We use the
// [ACM sdk v2](https://github.com/aws/aws-sdk-go-v2/tree/master/service/acm)
// to create the certificate. The client is created with the default
// credential chain loader.
func cogService() (*cip.CognitoIdentityProvider, error) {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		return nil, errors.Wrap(err, "could not load default config")
	}
	return cip.New(cfg), nil
}
