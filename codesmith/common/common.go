package common

import (
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/pkg/errors"
	"regexp"
)

var forgeSsmArnRegExp = regexp.MustCompile("^arn:aws.*:ssm:.*:.*:parameter" + SsmParameterPrefix + ".*$")

func IsForgeSsmParameterARN(s string) bool {
	return forgeSsmArnRegExp.MatchString(s)
}

var certificateArnRegExp = regexp.MustCompile("^arn:aws.*:acm:.*certificate/.*$")

func IsCertificateArn(s string) bool {
	return certificateArnRegExp.MatchString(s)
}

// Some of the custom resources are created in a different region that the
// cloudformation stack. Since the region propery is optional, we need to
// use the arn of an existing resource to detect if a change of region has
// happen: the region could have been undefined previously and gets
// suddenly defined or the other way around.
var regionExtractor = regexp.MustCompile(impo)

func ArnRegion(arn string) string {
	return regionExtractor.FindStringSubmatch(arn)[1]
}

func IsSameRegion(event cfn.Event, oldRegion, region string) bool {
	// 1. if the new region is the same as the old one, they are the same.
	if oldRegion == region {
		return true
	}
	// 2. else, if both are defined, they are not the same.
	if len(oldRegion) > 0 && len(region) > 0 {
		return false
	}
	// 3. else, we have a complicate case where either the old or the new region are implicit from the
	// region of the cloudformation stack.
	sdkRegion := ArnRegion(event.StackID)
	if sdkRegion == oldRegion || sdkRegion == region {
		return true
	}
	return false
}

// ## Request Type

func UnknownRequestType(event cfn.Event) (string, map[string]interface{}, error) {
	return event.PhysicalResourceID, nil, errors.Errorf("unknown request type %s", event.RequestType)
}
