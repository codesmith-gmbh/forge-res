// Names for some CloudFormation custom resources.
package common

// SSM Parameters
//
// Some of the custom CloudFormation resources create by the forge are implemented as SSM parameters in the parameter
// store.
const SsmParameterPrefix = "/codesmith-forge"

// Sequence naming
func SequenceParameterName(sequenceName string) string {
	return SsmParameterPrefix + "/Sequence" + sequenceName
}

// CogCondPreAuth naming
func CogCondPreAuthParameterName(userPoolId string, userPoolClientId string) string {
	return SsmParameterPrefix + "/CogCondPreAuth/" + userPoolId + "/" + userPoolClientId
}