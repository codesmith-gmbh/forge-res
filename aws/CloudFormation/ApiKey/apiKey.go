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
//   Type: Custom::ApiKey
//   Properties:
//     ServiceToken: !Import ForgeResources-ApiKey
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
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/codesmith-gmbh/cgc/cgcaws"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

// The lambda is started using the AWS lambda go sdk. The handler function
// does the actual work of creating the apikey. Cloudformation sends an
// event to signify that a resources must be created, updated or deleted.
func main() {
	cfg := cgcaws.MustConfig()
	p := newProc(cfg)
	lambda.Start(cfn.LambdaWrap(p.processEvent))
}

type proc struct {
	apg *apigateway.Client
	cf  *cloudformation.Client
}

func newProc(cfg aws.Config) *proc {
	return &proc{apg: apigateway.New(cfg), cf: cloudformation.New(cfg)}
}

type Properties struct {
	Ordinal string
}

func validateProperties(input map[string]interface{}) (Properties, error) {
	var properties Properties
	err := mapstructure.Decode(input, &properties)
	if err != nil {
		return properties, err
	}
	if properties.Ordinal == "" {
		return properties, errors.Wrap(err, "Ordinal is obligatory")
	}

	return properties, nil
}

// It is not possible to update the resource, if the ordinal changes, a new resource is allocated.
func (p *proc) processEvent(ctx context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	properties, err := validateProperties(event.ResourceProperties)
	if err != nil {
		return "", nil, err
	}
	switch event.RequestType {
	case cfn.RequestDelete:
		return p.deleteApiKey(ctx, event.PhysicalResourceID)
	case cfn.RequestUpdate, cfn.RequestCreate:
		return p.createApiKey(ctx, event, properties)
	default:
		return event.PhysicalResourceID, nil, errors.Errorf("unknown request type %s", event.RequestType)
	}
}

// To create the Api Key, we first retrieve the name of the stack and concatenate with the LogicalResourceId and Ordinal to create
// The Api Key Name.
func (p *proc) createApiKey(ctx context.Context, event cfn.Event, properties Properties) (string, map[string]interface{}, error) {
	stack, err := p.cf.DescribeStacksRequest(&cloudformation.DescribeStacksInput{
		StackName: &event.StackID,
	}).Send(ctx)
	if err != nil {
		return "", nil, errors.Wrapf(err, "Cannot retrieve the stack name for %s", event.StackID)
	}
	name := *stack.Stacks[0].StackName + "-" + event.LogicalResourceID + "-" + properties.Ordinal
	key, err := p.apg.CreateApiKeyRequest(&apigateway.CreateApiKeyInput{
		Name:    aws.String(name),
		Enabled: aws.Bool(true),
	}).Send(ctx)
	if err != nil {
		return "", nil, errors.Wrapf(err, "Cannot create the key with name %s", name)
	}
	return *key.Id, map[string]interface{}{"Secret": key.Value}, nil
}

func (p *proc) deleteApiKey(ctx context.Context, keyId string) (string, map[string]interface{}, error) {
	_, err := p.apg.DeleteApiKeyRequest(&apigateway.DeleteApiKeyInput{
		ApiKey: &keyId,
	}).Send(ctx)
	if err != nil {
		awsErr, ok := err.(awserr.RequestFailure)
		if !ok || awsErr.StatusCode() != 404 {
			return keyId, nil, errors.Wrapf(err, "could not delete the api key %s", keyId)
		}
	}
	return keyId, nil, nil
}
