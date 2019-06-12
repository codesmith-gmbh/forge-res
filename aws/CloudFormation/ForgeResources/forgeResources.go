package main

import (
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

const (
	ResponseStatusSuccess = "success"
)

var (
	serviceTokens = map[string]string{
		"Forge::ApiGateway::ApiKey":                          "ForgeResources-ApiKey",
		"Forge::Cognito::CondPreAuthSettings":                "ForgeResources-CogCondPreAuthSettings",
		"Forge::Cognito::IdentityProvider":                   "ForgeResources-CognitoIdentityProvider",
		"Forge::Cognito::UserPoolClientSettings":             "ForgeResources-CognitoUserPoolClientSettings",
		"Forge::Cognito::UserPoolDomain":                     "ForgeResources-CognitoUserPoolDomain",
		"Forge::CertificateManager::DnsCertificate":          "ForgeResources-DnsCertificate",
		"Forge::ECR::Cleanup":                                "ForgeResources-EcrCleanup",
		"Forge::ElasticLoadBalancingV2::ListenerRuleSwapper": "ForgeResources-ListenerRuleSwapper",
		"Forge::RDS::PostgresDatabase":                       "ForgeResources-PostgresDatabase",
		"Forge::RDS::DbInstanceResourceId":                   "ForgeResources-DbInstanceResourceId",
		"Forge::S3::Cleanup":                                 "ForgeResources-S3Cleanup",
		"Forge::Utils::Sequence":                             "ForgeResources-Sequence",
		"Forge::Utils::SequenceValue":                        "ForgeResources-SequenceValue",
	}
)

func main() {
	lambda.Start(processEvent)
}

func processEvent(src map[string]interface{}) (map[string]interface{}, error) {
	fmt.Printf("src: %v\n", src)
	response := newResponse(src)
	frag := src["fragment"].(map[string]interface{})
	frag, err := transform(frag)
	if err != nil {
		return nil, err
	}
	response["fragment"] = frag
	fmt.Printf("resp: %v\n", frag)
	return response, nil
}

func newResponse(src map[string]interface{}) map[string]interface{} {
	resp := make(map[string]interface{})
	resp["requestId"] = src["requestId"]
	resp["status"] = ResponseStatusSuccess
	return resp
}

func transform(src map[string]interface{}) (map[string]interface{}, error) {
	resources := src["Resources"].(map[string]interface{})
	for name, val := range resources {
		res := val.(map[string]interface{})
		resType := res["Type"].(string)
		if serviceToken, present := serviceTokens[resType]; present {
			res["Type"] = "AWS::CloudFormation::CustomResource"
			props := res["Properties"].(map[string]interface{})
			props["ServiceToken"] = map[string]string{"Fn::ImportValue": serviceToken}
		} else if resType == "Forge::ApiGateway::Redirector" {
			delete(resources, name)
			properties, err := validateApiGatewayRedirectorProperties(res["Properties"])
			if err != nil {
				return src, err
			}
			for _, gen := range []ResourceGenerator{
				apiGatewayRedirectorApiResource,
				domainNameRedirectorApiResource,
				basePathMappingRedirectorApiResource,
			} {
				addResourceTo(resources, name, properties, gen)
			}
		}
	}
	return src, nil
}

const (
	stageName = "redirector"
)

type ApiGatewayRedirectorProperties struct {
	CertificateArn interface{}
	DomainName     interface{}
	Location       interface{}
}

type ResourceGenerator func(string, ApiGatewayRedirectorProperties) (string, map[string]interface{})

func validateApiGatewayRedirectorProperties(input interface{}) (ApiGatewayRedirectorProperties, error) {
	var properties ApiGatewayRedirectorProperties
	if err := mapstructure.Decode(input, &properties); err != nil {
		return properties, err
	}
	if properties.Location == nil {
		return properties, errors.New("Location is obligatory")
	}
	return properties, nil
}

func addResourceTo(resources map[string]interface{}, name string, properties ApiGatewayRedirectorProperties,
	resourceGenerator ResourceGenerator) {
	resName, resource := resourceGenerator(name, properties)
	resources[resName] = resource
}

func apiGatewayRedirectorApiResource(name string, properties ApiGatewayRedirectorProperties) (string, map[string]interface{}) {
	apiName := apiName(name)
	res := make(map[string]interface{})
	res["Type"] = "AWS::Serverless::Api"
	res["Properties"] = map[string]interface{}{
		"Name": map[string]interface{}{
			"Fn::Sub": "${AWS::StackName}-" + apiName,
		},
		"StageName": stageName,
		"DefinitionBody": map[string]interface{}{
			"swagger": "2.0",
			"info":    map[string]interface{}{"version": "1.0"},
			"schemes": []string{"https"},
			"paths": map[string]interface{}{
				"/": map[string]interface{}{
					"get": map[string]interface{}{
						"consumes": []string{"application/json"},
						"responses": map[string]interface{}{
							"301": map[string]interface{}{
								"description": "301 response",
								"headers": map[string]interface{}{
									"Location": map[string]interface{}{
										"type": "string",
									},
								},
							},
						},
						"x-amazon-apigateway-integration": map[string]interface{}{
							"responses": map[string]interface{}{
								"301": map[string]interface{}{
									"statusCode": "301",
									"responseParameters": map[string]interface{}{
										"method.response.header.Location": map[string]interface{}{
											"Fn::Sub": []interface{}{
												"'${Location}'",
												map[string]interface{}{
													"Location": properties.Location,
												},
											},
										},
									},
								},
							},
							"requestTemplates": map[string]interface{}{
								"application/json": "{\"statusCode\": 301}",
							},
							"passthroughBehavior": "when_no_match",
							"type":                "mock",
						},
					},
				},
			},
			"definitions": map[string]interface{}{
				"Empty": map[string]interface{}{
					"type":  "object",
					"title": "Empty Schema",
				},
			},
		},
	}
	return apiName, res
}

func domainNameRedirectorApiResource(name string, properties ApiGatewayRedirectorProperties) (string, map[string]interface{}) {
	res := make(map[string]interface{})
	res["Type"] = "AWS::ApiGateway::DomainName"
	res["Properties"] = map[string]interface{}{
		"CertificateArn":        properties.CertificateArn,
		"DomainName":            properties.DomainName,
		"EndpointConfiguration": map[string]interface{}{"Types": []string{"EDGE"}},
	}
	return domainName(name), res
}

func basePathMappingRedirectorApiResource(name string, _ ApiGatewayRedirectorProperties) (string, map[string]interface{}) {
	res := make(map[string]interface{})
	res["Type"] = "AWS::ApiGateway::BasePathMapping"
	res["Properties"] = map[string]interface{}{
		"DomainName": map[string]interface{}{"Ref": domainName(name)},
		"RestApiId":  map[string]interface{}{"Ref": apiName(name)},
		"Stage":      stageName,
	}
	return basePathMappingName(name), res
}

func apiName(name string) string {
	return name + "Api"
}

func domainName(name string) string {
	return name + "DomainName"
}

func basePathMappingName(name string) string {
	return name + "BasePathMapping"
}
