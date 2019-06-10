package main

import (
	"context"
	"github.com/codesmith-gmbh/cgc/cgctesting"
	"testing"
)

func TestDeletionUnexistingApiKey(t *testing.T) {
	cfg := cgctesting.MustTestConfig()
	p := newProcFromConfig(cfg)
	var apiKeyId = "??????"
	_, _, err := p.deleteApiKey(context.TODO(), apiKeyId)
	if err != nil {
		t.Error(err)
	}
}
