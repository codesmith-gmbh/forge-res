package main

import (
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/codesmith-gmbh/forge/aws/testCommon"
	"testing"
)

func TestDeleteUnexistingIdentiyProvider(t *testing.T) {
	cfg := testCommon.MustTestConfig()
	p := newProc(cfg)
	properties := Properties{UserPoolId: "a_aaaa", ProviderName: string(cognitoidentityprovider.IdentityProviderTypeTypeGoogle)}
	_, _, err := p.deleteIdentityProvider(
		cfn.Event{PhysicalResourceID: physicalResourceID(properties)},
		properties,
	)
	if err != nil {
		t.Error(err)
	}
}
