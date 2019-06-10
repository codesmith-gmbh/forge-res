package main

import (
	"context"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/codesmith-gmbh/cgc/cgctesting"
	"testing"
)

func TestDeleteUnexisting(t *testing.T) {
	cfg := cgctesting.MustTestConfig()
	p := newProcFromConfig(cfg)
	_, _, err := p.deleteDomain(
		context.TODO(),
		cfn.Event{PhysicalResourceID: "userpool.test.com"},
		Properties{UserPoolId: "a_aaaaa"})
	if err != nil {
		t.Error(err)
	}
}
