package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"io/ioutil"
	"testing"
)

func TestDnsCertificatePropertiesWithRegion(t *testing.T) {
	var input map[string]interface{}
	inputJSON, err := ioutil.ReadFile("./testdata/dnsCertificateProperties.json")
	if err != nil {
		t.Fatal(err)
	}
	err = json.Unmarshal(inputJSON, &input)
	if err != nil {
		t.Fatal(err)
	}
	p, err := decodeProperties(input)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", p)
}

func TestDnsCertificatePropertiesWithoutRegion(t *testing.T) {
	var input map[string]interface{}
	inputJSON, err := ioutil.ReadFile("./testdata/dnsCertificateProperties.json")
	if err != nil {
		t.Fatal(err)
	}
	err = json.Unmarshal(inputJSON, &input)
	if err != nil {
		t.Fatal(err)
	}
	delete(input, "Region")
	p, err := decodeProperties(input)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", p)
}

func TestOnlyTagsChangedCorrectness(t *testing.T) {
	if !onlyTagsChanged(
		cfn.Event{},
		Properties{
			DomainName:              "test.com",
			Region:                  "us-east-1",
			SubjectAlternativeNames: []string{"hello1.test.com", "hello2.test.com"},
			Tags:                    []acm.Tag{tag("key", "value1")},
		},
		Properties{
			DomainName:              "test.com",
			Region:                  "us-east-1",
			SubjectAlternativeNames: []string{"hello2.test.com", "hello1.test.com"},
			Tags:                    []acm.Tag{tag("key", "value2")},
		}) {
		t.Fatal("TestOnlyTagsChangedCorrectness")
	}
}

func TestOnlyTagsChangedCompleteness(t *testing.T) {
	for i, test := range []struct{ prop1, prop2 Properties }{
		// test 0
		{Properties{
			DomainName:              "test.com",
			Region:                  "us-east-1",
			SubjectAlternativeNames: []string{"hello1.test.com", "hello2.test.com"},
			Tags:                    []acm.Tag{tag("key", "value1")},
		}, Properties{
			DomainName:              "hello3.test.com",
			Region:                  "us-east-1",
			SubjectAlternativeNames: []string{"hello2.test.com", "hello1.test.com"},
			Tags:                    []acm.Tag{tag("key", "value2")},
		}},
		// test 1
		{Properties{
			DomainName:              "test.com",
			Region:                  "us-east-1",
			SubjectAlternativeNames: []string{"hello1.test.com", "hello2.test.com"},
			Tags:                    []acm.Tag{tag("key", "value1")},
		}, Properties{
			DomainName:              "test.com",
			Region:                  "eu-west-1",
			SubjectAlternativeNames: []string{"hello2.test.com", "hello1.test.com"},
			Tags:                    []acm.Tag{tag("key", "value2")},
		}},
		// test 2
		{Properties{
			DomainName:              "test.com",
			Region:                  "us-east-1",
			SubjectAlternativeNames: []string{"hello1.test.com", "hello3.test.com"},
			Tags:                    []acm.Tag{tag("key", "value1")},
		}, Properties{
			DomainName:              "test.com",
			Region:                  "us-east-1",
			SubjectAlternativeNames: []string{"hello2.test.com", "hello1.test.com"},
			Tags:                    []acm.Tag{tag("key", "value2")},
		}},
		// test 3
		{Properties{
			DomainName:              "test.com",
			Region:                  "us-east-1",
			SubjectAlternativeNames: []string{"hello1.test.com", "hello2.test.com", "hello3.test.com"},
			Tags:                    []acm.Tag{tag("key", "value1")},
		}, Properties{
			DomainName:              "test.com",
			Region:                  "us-east-1",
			SubjectAlternativeNames: []string{"hello2.test.com", "hello1.test.com"},
			Tags:                    []acm.Tag{tag("key", "value2")},
		}},
	} {
		if onlyTagsChanged(cfn.Event{}, test.prop1, test.prop2) {
			t.Errorf("TestOnlyTagsChangedCompleteness %d, %+v, %+v", i, test.prop1, test.prop2)
		}
	}
}

func tag(key string, val string) acm.Tag {
	return acm.Tag{Key: &key, Value: &val}
}
