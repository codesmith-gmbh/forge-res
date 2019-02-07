package main

import (
	"encoding/json"
	"fmt"
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
	p, err := properties(input)
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
	p, err := properties(input)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", p)
}
