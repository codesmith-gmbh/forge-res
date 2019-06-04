package main

import (
	"github.com/codesmith-gmbh/forge/aws/common"
	"github.com/codesmith-gmbh/forge/aws/testCommon"
	"testing"
)

func TestDeletionUnexistingParameter(t *testing.T) {
	cfg := testCommon.MustTestConfig()
	p := newProc(cfg)
	_, _, err := p.deleteParameter(common.CogCondPreAuthParameterName("???", "???"))
	if err != nil {
		t.Error(err)
	}
}
