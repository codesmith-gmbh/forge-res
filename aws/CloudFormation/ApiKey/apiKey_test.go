package main

import (
	"context"
	"github.com/codesmith-gmbh/forge/aws/testCommon"
	"testing"
)

func TestDeletionUnexistingApiKey(t *testing.T) {
	cfg := testCommon.MustTestConfig()
	p := newProc(cfg)
	var apiKeyId = "??????"
	_, _, err := p.deleteApiKey(context.TODO(), apiKeyId)
	if err != nil {
		t.Error(err)
	}
}
