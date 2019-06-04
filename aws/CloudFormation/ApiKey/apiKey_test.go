package main

import (
	"github.com/codesmith-gmbh/forge/aws/testCommon"
	"testing"
)

func TestDeletionUnexistingApiKey(t *testing.T) {
	cfg := testCommon.MustTestConfig()
	p := newProc(cfg)
	var apiKeyId = "??????"
	_, _, err := p.deleteApiKey(apiKeyId)
	if err != nil {
		t.Error(err)
	}
}
