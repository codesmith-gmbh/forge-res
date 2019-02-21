package main

import (
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/codesmith-gmbh/forge/aws/common"
	"testing"
)

func TestDeletionUnexistingApiKey(t *testing.T) {
	cfg := common.MustConfig(external.WithSharedConfigProfile("codesmith"))
	p := newProc(cfg)
	var apiKeyId = "??????"
	_, _, err := p.deleteApiKey(apiKeyId)
	if err != nil {
		t.Error(err)
	}
}
