// # CogCondPreAuth
//
// The CogCondPreAuth lambda function is as a hook for the PreAuth event of a Cognito User and is used in conjunction
// with the cogCondPreAuthSettings custom Cloudformation resource for its configuration.
//
// The CogCondPreAuth lambda function will allow/deny authentication based on the email address of the user logging in.
// Domains and individual email addresses can be whitelisted (via the cogCondPreAuthSettings custom CloudFormation
// resource.
//
package main

import (
	"context"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/codesmith-gmbh/cgc/cgcaws"
	"github.com/codesmith-gmbh/cgc/cgclog"
	"github.com/pkg/errors"
	"strings"
)

// CognitoEventUserPoolsPreSignupRequest contains the request portion of a PreAuth event
type CognitoEventUserPoolsPreAuth struct {
	events.CognitoEventUserPoolsHeader
	Request  events.CognitoEventUserPoolsPreSignupRequest `json:"request"`
	Response map[string]interface{}                       `json:"response"`
}

type Settings struct {
	All     bool     `json:"all"`
	Domains []string `json:"domains"`
	Emails  []string `json:"emails"`
}

var (
	zero = CognitoEventUserPoolsPreAuth{}
	log  = cgclog.MustSugaredLogger()
)

func main() {
	defer cgclog.SyncSugaredLogger(log)
	cfg := cgcaws.MustConfig()
	p := newProc(cfg)
	lambda.Start(p.processEvent)
}

type proc struct {
	ssm *ssm.Client
}

func newProc(cfg aws.Config) *proc {
	return &proc{ssm: ssm.New(cfg)}
}

func (p *proc) processEvent(ctx context.Context, event CognitoEventUserPoolsPreAuth) (CognitoEventUserPoolsPreAuth, error) {
	log.Debugw("received pre auth event", "event", event)

	// 1. we fetch the Settings for the given user pool
	userPoolId := event.UserPoolID
	clientId := event.CallerContext.ClientID
	settings, err := p.fetchSettings(ctx, userPoolId, clientId)
	if err != nil {
		return zero, err
	}

	// 2. we get the email and the domain name from the event.
	email, domain, err := emailAndDomainOfUser(event)
	if err != nil {
		return zero, err
	}

	// 3. To a accept an authentication request, one of the following condition must be true:
	//    a. The `All` flag is set to true
	//    b. The domain name of the user is contained in the list of whitelisted domain names.
	//    c. The email of the user is contained in the list of whitelisted domain names.
	if !(settings.All || in(settings.Domains, domain) || in(settings.Emails, email)) {
		log.Infow("user not authorized", "email", email)
		return zero, errors.New("not authorized")
	}
	log.Infow("user authorized", "email", email)
	return event, nil
}

func (p *proc) fetchSettings(ctx context.Context, userPoolId, clientId string) (Settings, error) {
	var settings Settings
	parameterName := "/codesmith-forge/CogCondPreAuth/" + userPoolId + "/" + clientId
	log.Debugw("fetch settings", "parameterName", parameterName)
	parameter, err := p.ssm.GetParameterRequest(&ssm.GetParameterInput{
		Name: &parameterName,
	}).Send(ctx)
	if err != nil {
		return settings, errors.Wrapf(err, "error fetching the parameter %s", parameterName)
	}
	if parameter.Parameter == nil || parameter.Parameter.Value == nil {
		return settings, errors.Errorf("no configuration for the client %s of user pool %s", clientId, userPoolId)
	}
	err = json.Unmarshal([]byte(*parameter.Parameter.Value), &settings)
	if err != nil {
		return settings, errors.Wrapf(err, "invalid settings for the client %s of user pool %s", clientId, userPoolId)
	}
	log.Debugw("settings fetched", "settings", settings)
	return settings, nil
}

func emailAndDomainOfUser(event CognitoEventUserPoolsPreAuth) (string, string, error) {
	email := event.Request.UserAttributes["email"]
	splitted := strings.Split(email, "@")
	if len(splitted) != 2 {
		return "", "", errors.Errorf("invalid email: %s", email)
	}
	domain := splitted[1]
	log.Debugw("email domain of user", "email", email, "domain", domain)
	return email, domain, nil
}

func in(strings []string, val string) bool {
	for _, s := range strings {
		if val == s {
			return true
		}
	}
	return false
}
