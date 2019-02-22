package testCommon

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/codesmith-gmbh/forge/aws/common"
	"os"
)

func MustTestConfig() aws.Config {
	codeBuildId := os.Getenv("CODEBUILD_BUILD_ID")
	if codeBuildId == "" {
		testProfile := os.Getenv("FORGE_TEST_PROFILE")
		if testProfile == "" {
			panic("the env var FORGE_TEST_PROFILE is not defined")
		}
		return common.MustConfig(external.WithSharedConfigProfile(testProfile))
	}
	return common.MustConfig()
}
