package acm

import "testing"

func TestCertificateRegion(t *testing.T) {
	// 1. Correctness
	for i, test := range []struct {
		arn, region string
	}{
		{arn: "arn:aws:acm:eu-west-1:account:certificate/12345678-1234-1234-1234-123456789012", region: "eu-west-1"},
		{arn: "arn:aws:acm:us-east-1:account:certificate/12345678-1234-1234-1234-123456789012", region: "us-east-1"},
	} {
		region, err := certificateRegion(test.arn)
		if err != nil {
			t.Errorf("error in correctness test case %d, %v, %s", i, test, err)
		}
		if region != test.region {
			t.Errorf("correctness test case %d, expected: %s, got: %s, test case %v", i, test.region, region, test)
		}
	}

	// 2. Completeness
	for i, test := range []string{
		"StackId-ResourceId-12345678",
	} {
		_, err := certificateRegion(test)
		if err == nil {
			t.Errorf("completeness test case %d, %s", i, test)
		}
	}
}
