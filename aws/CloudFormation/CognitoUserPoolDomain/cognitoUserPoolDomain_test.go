package main

import (
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/codesmith-gmbh/forge/aws/testCommon"
	"testing"
)

func TestDeleteUnexisting(t *testing.T) {
	cfg := testCommon.MustTestConfig()
	p := newProc(cfg)
	_, _, err := p.deleteDomain(cfn.Event{PhysicalResourceID: "userpool.test.com"}, Properties{UserPoolId: "a_aaaaa"})
	if err != nil {
		t.Error(err)
	}
}
