package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/codesmith-gmbh/forge/aws/testCommon"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"testing"
)

func TestDnsPropertiesJson(t *testing.T) {
	var input map[string]interface{}
	inputJSON, err := ioutil.ReadFile("./testdata/cogclientset.json")
	if err != nil {
		t.Fatal(err)
	}
	err = json.Unmarshal(inputJSON, &input)
	if err != nil {
		t.Fatal(err)
	}
	p, err := validateProperties(input)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", p)
}

func TestDnsPropertiesYaml(t *testing.T) {
	var input map[string]interface{}
	inputYAML, err := ioutil.ReadFile("./testdata/cogclientset.yaml")
	if err != nil {
		t.Fatal(err)
	}
	err = yaml.Unmarshal(inputYAML, &input)
	if err != nil {
		t.Fatal(err)
	}
	p, err := validateProperties(input)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", p)
}

func TestDeleteUnexistingResource(t *testing.T) {
	cfg := testCommon.MustTestConfig()
	p := newProc(cfg)
	_, _, err := p.deleteCognitoUserPoolClientSettings(
		cfn.Event{PhysicalResourceID: "aaaaaaaaaaaa"},
		Properties{UserPoolId: "a_aaaaaaaaaaa", UserPoolClientId: "aaaaaaaaaaaa"},
	)
	if err != nil {
		t.Error(err)
	}
}
