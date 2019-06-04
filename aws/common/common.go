package common

import (
	"fmt"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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
var regionExtractor = regexp.MustCompile("arn:aws:(?:.*?):(.*?):")

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

// ## Logging functions based on zap

func MustSugaredLogger() *zap.SugaredLogger {
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	logger, err := config.Build()
	if err != nil {
		panic(err)
	}
	return logger.Sugar()
}

func SyncSugaredLogger(logger *zap.SugaredLogger) {
	if err := logger.Sync(); err != nil {
		fmt.Printf("could not sync sugared logger: %v", err)
	}
}

// ## Standard AWS config

func MustConfig(configs ...external.Config) aws.Config {
	cfg, err := external.LoadDefaultAWSConfig(configs...)
	if err != nil {
		panic(err)
	}
	return cfg
}

// ## Request Type

func UnknownRequestType(event cfn.Event) (string, map[string]interface{}, error) {
	return event.PhysicalResourceID, nil, errors.Errorf("unknown request type %s", event.RequestType)
}
