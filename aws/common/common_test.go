package common

import (
	"github.com/aws/aws-lambda-go/cfn"
	"testing"
)

func TestIsForgeSsmARN(t *testing.T) {
	// 1. Correctness
	for i, test := range []string{
		"arn:aws:ssm:region:account-id:parameter/codesmith-forge/Sequence/a",
	} {
		if !IsForgeSsmParameterARN(test) {
			t.Errorf("correctness test case %d, %s", i, test)
		}
	}

	// 2. Completeness
	for i, test := range []string{
		"StackId-ResourceId-12345678",
		"arn:aws:ssm:region:account-id:parameter/application",
	} {
		if IsForgeSsmParameterARN(test) {
			t.Errorf("completeness test case %d, %s", i, test)
		}
	}
}

func TestIsCertificateArn(t *testing.T) {
	// 1. Correctness
	for i, test := range []string{
		"arn:aws:acm:region:account:certificate/12345678-1234-1234-1234-123456789012",
	} {
		if !IsCertificateArn(test) {
			t.Errorf("correctness test case %d, %s", i, test)
		}
	}

	// 2. Completeness
	for i, test := range []string{
		"StackId-ResourceId-12345678",
	} {
		if IsCertificateArn(test) {
			t.Errorf("completeness test case %d, %s", i, test)
		}
	}
}

func TestIsSameRegion(t *testing.T) {
	// 1. Correctness
	for i, test := range []struct {
		event             cfn.Event
		oldRegion, region string
	}{
		// test 0
		{oldRegion: "", region: ""},
		// test 1
		{oldRegion: "us-east-1", region: "us-east-1"},
		// test 2
		{event: cfn.Event{StackID: "arn:aws:cloudformation:us-west-2:123456789012:stack/teststack/51af3dc0-da77-11e4-872e-1234567db123"},
			oldRegion: "", region: "us-west-2"},
		// test 3
		{event: cfn.Event{StackID: "arn:aws:cloudformation:us-west-2:123456789012:stack/teststack/51af3dc0-da77-11e4-872e-1234567db123"},
			oldRegion: "us-west-2", region: ""},
	} {
		if !IsSameRegion(test.event, test.oldRegion, test.region) {
			t.Errorf("Test case %d, %+v, %s, %s", i, test.event, test.oldRegion, test.region)
		}
	}

	// 2. Completeness
	for i, test := range []struct {
		event             cfn.Event
		oldRegion, region string
	}{
		// test 0
		{oldRegion: "us-east-1", region: "us-east-2"},
		// test 1
		{event: cfn.Event{StackID: "arn:aws:cloudformation:us-west-2:123456789012:stack/teststack/51af3dc0-da77-11e4-872e-1234567db123"},
			oldRegion: "", region: "us-west-1"},
		// test 2
		{event: cfn.Event{StackID: "arn:aws:cloudformation:us-west-2:123456789012:stack/teststack/51af3dc0-da77-11e4-872e-1234567db123"},
			oldRegion: "us-west-1", region: ""},
	} {
		if IsSameRegion(test.event, test.oldRegion, test.region) {
			t.Errorf("Test case %d, %+v, %s, %s", i, test.event, test.oldRegion, test.region)
		}
	}
}
