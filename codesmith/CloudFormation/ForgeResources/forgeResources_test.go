package main

import (
	"fmt"
	"testing"
)

func TestApiGatewayRedirectorApiResource(t *testing.T) {
	_, text := apiGatewayRedirectorApiResource(
		"Redirector",
		ApiGatewayRedirectorProperties{Location: map[string]interface{}{"Fn::Ref": "ParamLocation"}})
	fmt.Printf("%v\n", text)
}
