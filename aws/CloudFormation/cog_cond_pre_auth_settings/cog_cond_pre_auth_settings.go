// # Sequence Generator
//
// The `cog_cond_pre_auth_settings` custom resource is used to configure the
// `cog_cond_pre_auth` cognito trigger lambda with a SSM parameter.
//
// For more information, consult the documentation of the `cog_cond_pre_auth`
// cognito trigger
//
// ## Syntax
//
// To create an `cog_cond_pre_auth_settings` resource, add the following resource
// to your cloudformation template (yaml notation, json is similar)
//
// ```yaml
// UserPoolPreAuthSettings:
//   Type: Custom::CogCondPreAuthSettings
//   Properties:
//     ServiceToken:
//       Fn::ImportValue:
//         !Sub ${HyperdriveCore}-CogCondPreAuthSettings
//     UserPoolId: <userpool-id>
//	   UserPoolCliendId: <userpoolclient-id>
//     All: false
//     Domains:
//     - test.com
//     Emails:
//     - stan@test2.com
// ```
//
// ## Properties
//
// `UserPoolId`
//
// > The id of the user pool to which the trigger `cog_cond_pre_auth` is attached.
//
// _Type_: String
//
// _Required_: Yes
//
// _Update Requires_: replacement
//
//
// `UserPoolCliendId`
//
// > The id of the user pool client id used to login users.
//
// _Type_: String
//
// _Required_: Yes
//
// _Update Requires_: replacement
//
//
// `All`:
//
// > A flag to configure whether all users can authenticate via the given client.
//
// _Type_: boolean
//
// _Required_: no (default: false)
//
// _Update Requires_: no interruption
//
//
// `Domains`:
//
// > A list of email domains to whitelist
//
// _Type_: List of Strings
//
// _Required_: no (default: [])
//
// _Update Requires_: no interruption
//
//
// `Emails`:
//
// > A list of individual emails to whitelist
//
// _Type_: List of Strings
//
// _Required_: no (default: [])
//
// _Update Requires_: no interruption
//
// ## Return Values
//
// `Ref`
//
// The `Ref` intrinsic function gives the name of the created SSM parameter
package main

import (
	"context"
	"encoding/json"
	"github.com/DEEP-IMPACT-AG/hyperdrive/common"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	awsssm "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"strconv"
)

var ssm *awsssm.SSM

func main() {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		panic(err)
	}
	ssm = awsssm.New(cfg)
	lambda.Start(cfn.LambdaWrap(processEvent))
}

// The SequenceProperties is the main data structure for the resource and
// is defined as a go struct. The struct mirrors the properties as defined above.
// We use the library [mapstructure](https://github.com/mitchellh/mapstructure) to
// decode the generic map from the cloudformation event to the struct.
type CogCondPreAuthSettingsProperties struct {
	UserPoolId       string
	UserPoolClientId string
	All              string
	Domains          []string
	Emails           []string
}

func cogCondPreAuthSettingsProperties(input map[string]interface{}) (CogCondPreAuthSettingsProperties, error) {
	var properties CogCondPreAuthSettingsProperties
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

func processEvent(ctx context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	properties, err := cogCondPreAuthSettingsProperties(event.ResourceProperties)
	if err != nil {
		return "", nil, err
	}
	switch event.RequestType {
	case cfn.RequestDelete:
		if !common.IsFailurePhysicalResourceId(event.PhysicalResourceID) {
			_, err := ssm.DeleteParameterRequest(&awsssm.DeleteParameterInput{
				Name: &event.PhysicalResourceID,
			}).Send()
			if err != nil {
				return event.PhysicalResourceID, nil, errors.Wrapf(err, "could not delete the parameter %s", event.PhysicalResourceID)
			}
		}
		return event.PhysicalResourceID, nil, nil
	case cfn.RequestCreate:
		return putParameter(ssm, properties)
	case cfn.RequestUpdate:
		return putParameter(ssm, properties)
	default:
		return "", nil, errors.Errorf("unknown request type %s", event.RequestType)
	}
}

func putParameter(ssm *awsssm.SSM, properties CogCondPreAuthSettingsProperties) (string, map[string]interface{}, error) {
	overwrite := true
	parameterName := "/hyperdrive/cog_cond_pre_auth/" + properties.UserPoolId + "/" + properties.UserPoolClientId
	all, err := strconv.ParseBool(properties.All)
	if err != nil {
		return "", nil, errors.Wrapf(err, "All must be a booleand: %s", properties.All)
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
	_, err = ssm.PutParameterRequest(&awsssm.PutParameterInput{
		Overwrite: &overwrite,
		Name:      &parameterName,
		Type:      awsssm.ParameterTypeString,
		Value:     &dataText,
	}).Send()
	if err != nil {
		return "", nil, errors.Wrapf(err, "could not put the parameter %s", parameterName)
	}
	return parameterName, nil, nil
}
