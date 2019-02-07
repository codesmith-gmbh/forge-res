// # CF Api Key
//
// To bind an Api of the API Gateway to a custom Cloud Front distribution requires the use of a API Key to
// protect the native end point of the API and to make sure that the API is available only via the Cloud Front
// distribution.
// Unfortunately, the out-of-the-box resource `AWS::ApiGateway::ApiKey` does not allow to extract the key secret.
// This resource creates api keys and export the key secret.
//
// ## Syntax
// To create a new api key, add the following resource to your template
//
// ```yaml
// MyCfApiKey:
//   Type: Custom::CfApiKey
//   Properties:
//     ServiceToken:
//       Fn::ImportValue:
//         !Sub ${HyperdriveCore}-CfApiKey
//     Ordinal: <number>
// ```
//
// ## Properties
//
// `Ordinal`
//
// > The name of the API keys is created automatically from the Stack Name and the Ordinal. By making the Ordinal
// > a parameter of the stack, one can easily rotate the keys. You can automate the key rotation with the
// > AWS Lambda `RotateCfApiKey`.
//
// > _Type_: Number
// >
// > _Required: Yes
// >
// > _Update Requires_: Replacement
package main

import (
	"context"
	"github.com/DEEP-IMPACT-AG/hyperdrive/common"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"strconv"
)

var cf *cloudformation.CloudFormation
var apg *apigateway.APIGateway

// The lambda is started using the AWS lambda go sdk. The handler function
// does the actual work of creating the apikey. Cloudformation sends an
// event to signify that a resources must be created, updated or deleted.
func main() {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		panic(err)
	}
	apg = apigateway.New(cfg)
	cf = cloudformation.New(cfg)
	lambda.Start(cfn.LambdaWrap(processEvent))
}

type ApiKeyProperties struct {
	Ordinal string
}

func apiKeyProperties(input map[string]interface{}) (ApiKeyProperties, error) {
	var properties ApiKeyProperties
	err := mapstructure.Decode(input, &properties)
	if err != nil {
		return properties, err
	}
	_, err = strconv.ParseUint(properties.Ordinal, 10, 64)
	if err != nil {
		return properties, errors.Wrapf(err, "Ordinal is obligatory and must be a uint64: %s", properties.Ordinal)
	}

	return properties, nil
}

// It is not possible to update the resource, if the ordinal changes, a new resource is allocated.
func processEvent(ctx context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	properties, err := apiKeyProperties(event.ResourceProperties)
	if err != nil {
		return "", nil, err
	}
	switch event.RequestType {
	case cfn.RequestDelete:
		if !common.IsFailurePhysicalResourceId(event.PhysicalResourceID) {
			_, err := apg.DeleteApiKeyRequest(&apigateway.DeleteApiKeyInput{
				ApiKey: &event.PhysicalResourceID,
			}).Send()
			if err != nil {
				return event.PhysicalResourceID, nil, errors.Wrapf(err, "could not delete the api key %s", event.PhysicalResourceID)
			}
		}
		return event.PhysicalResourceID, nil, nil
	case cfn.RequestUpdate:
		return createApiKey(event.StackID, properties)
	case cfn.RequestCreate:
		return createApiKey(event.StackID, properties)
	default:
		return event.PhysicalResourceID, nil, errors.Errorf("unknown request type %s", event.RequestType)
	}
}

// To create the Api Key, we first retrieve the name of the stack and concatenate with the Ordinal to create
// The Api Key Name.
func createApiKey(stackId string, properties ApiKeyProperties) (string, map[string]interface{}, error) {
	stack, err := cf.DescribeStacksRequest(&cloudformation.DescribeStacksInput{
		StackName: &stackId,
	}).Send()
	if err != nil {
		return "", nil, errors.Wrapf(err, "Cannot retrieve the stack name for %s", stackId)
	}
	name := *stack.Stacks[0].StackName + "-" + properties.Ordinal
	enabled := true
	key, err := apg.CreateApiKeyRequest(&apigateway.CreateApiKeyInput{
		Name:    &name,
		Enabled: &enabled,
	}).Send()
	if err != nil {
		return "", nil, errors.Wrapf(err, "Cannot create the key with name %s", name)
	}
	return *key.Id, map[string]interface{}{"Secret": key.Value}, nil
}
