package main

import (
	"context"
	"github.com/codesmith-gmbh/cgc/cgctesting"
	"github.com/codesmith-gmbh/forge/aws/common"
	"testing"
)

func TestDeletionUnexistingParameter(t *testing.T) {
	cfg := cgctesting.MustTestConfig()
	p := newProc(cfg)
	_, _, err := p.deleteParameter(context.TODO(), common.CogCondPreAuthParameterName("???", "???"))
	if err != nil {
		t.Error(err)
	}
}
