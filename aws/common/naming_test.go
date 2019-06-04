package common

import "testing"

func TestExtractStackId(t *testing.T) {
	// 1. correctness
	for i, test := range []struct{ arn, id string }{
		{arn: "arn:aws:cloudformation:eu-west-1:1234567890:stack/Test/1f2fb450-3767-11e9-a02b-0a9391483dc6", id: "Test/1f2fb450-3767-11e9-a02b-0a9391483dc6"},
	} {
		id, err := ExtractStackId(test.arn)
		if err != nil {
			t.Errorf("case %d with error %s", i, err)
		}
		if test.id != id {
			t.Errorf("case %d, expecting %s got %s", i, test.id, id)
		}
	}
	// 2. completeness
	for i, test := range []string{
		"",
		"Test/1f2fb450-3767-11e9-a02b-0a9391483dc6",
		"arn:aws:iam::123456789012:user/TestUser",
	} {
		_, err := ExtractStackId(test)
		if err == nil {
			t.Errorf("case %d without error", i)
		}
	}
}

func TestExtractStackName(t *testing.T) {
	// 1. correctness
	for i, test := range []struct{ arn, id string }{
		{arn: "arn:aws:cloudformation:eu-west-1:1234567890:stack/Test/1f2fb450-3767-11e9-a02b-0a9391483dc6", id: "Test"},
	} {
		id, err := ExtractStackName(test.arn)
		if err != nil {
			t.Errorf("case %d with error %s", i, err)
		}
		if test.id != id {
			t.Errorf("case %d, expecting %s got %s", i, test.id, id)
		}
	}
	// 2. completeness
	for i, test := range []string{
		"",
		"Test/1f2fb450-3767-11e9-a02b-0a9391483dc6",
		"arn:aws:iam::123456789012:user/TestUser",
	} {
		_, err := ExtractStackName(test)
		if err == nil {
			t.Errorf("case %d without error", i)
		}
	}
}

