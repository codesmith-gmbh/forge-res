package main

import (
	"context"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/codesmith-gmbh/cgc/cgctesting"
	"testing"
)

func TestDeleteUnexistingIdentiyProvider(t *testing.T) {
	cfg := cgctesting.MustTestConfig()
	p := newProcFromConfig(cfg)
	properties := Properties{UserPoolId: "a_aaaa", ProviderName: string(cognitoidentityprovider.IdentityProviderTypeTypeGoogle)}
	_, _, err := p.deleteIdentityProvider(
		context.TODO(),
		cfn.Event{PhysicalResourceID: physicalResourceID(properties)},
		properties,
	)
	if err != nil {
		t.Error(err)
	}
}
