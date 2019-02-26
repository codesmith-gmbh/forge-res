// Names for some CloudFormation custom resources.
package common

import (
	"github.com/pkg/errors"
	"regexp"
)

// SSM Parameters
//
// Some of the custom CloudFormation resources create by the forge are implemented as SSM parameters in the parameter
// store.
const SsmParameterPrefix = "/codesmith-forge"

// Dns Certificate SNS Message memory
func DnsCertificeSnsMessageIdParameterName(stackArn string, logicalResourceID string) (string, error) {
	stackId, err := ExtractStackId(stackArn)
	if err != nil {
		return "", err
	}
	return SsmParameterPrefix + "/DnsCertificateSnsMessageId/" + stackId + "/" + logicalResourceID, nil
}

var stackIdRegExp = regexp.MustCompile("^arn:.*:cloudformation:.*:.*:stack/(.*)$")

func ExtractStackId(stackArn string) (string, error) {
	submatch := stackIdRegExp.FindStringSubmatch(stackArn)
	if len(submatch) != 2 {
		return "", errors.Errorf("Could not extract stackId from stack arn %s", stackArn)
	}
	return submatch[1], nil
}

var stackNameRegExp = regexp.MustCompile("^arn:.*:cloudformation:.*:.*:stack/(.*?)/.*$")

func ExtractStackName(stackArn string) (string, error) {
	submatch := stackNameRegExp.FindStringSubmatch(stackArn)
	if len(submatch) != 2 {
		return "", errors.Errorf("could not extract stackName for stack arn %s", stackArn)
	}
	return submatch[1], nil
}

// Sequence naming
func SequenceParameterName(sequenceName string) string {
	return SsmParameterPrefix + "/Sequence" + sequenceName
}

// CogCondPreAuth naming
func CogCondPreAuthParameterName(userPoolId string, userPoolClientId string) string {
	return SsmParameterPrefix + "/CogCondPreAuth/" + userPoolId + "/" + userPoolClientId
}
