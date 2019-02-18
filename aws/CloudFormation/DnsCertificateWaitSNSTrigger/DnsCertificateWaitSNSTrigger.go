package main

import (
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/codesmith-gmbh/forge/aws/common"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"os"
)

var log = common.MustSugaredLogger()

type proc struct {
	step *sfn.SFN
}

var stateMachineArn = os.Getenv("STATE_MACHINE_ARN")

func main() {
	defer common.SyncSugaredLogger(log)
	cfg := common.MustConfig()
	p := &proc{step: sfn.New(cfg)}
	lambda.Start(p.processEvent)
}

func (p *proc) processEvent(event events.SNSEvent) error {
	for _, rec := range event.Records {
		err := p.processRecord(rec)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *proc) processRecord(rec events.SNSEventRecord) error {
	name, err := executionName()
	if err != nil {
		return err
	}
	exe, err := p.step.StartExecutionRequest(&sfn.StartExecutionInput{
		Input:           &rec.SNS.Message,
		Name:            &name,
		StateMachineArn: &stateMachineArn,
	}).Send()
	if err != nil {
		return errors.Wrapf(err, "could not start the execution for the sns record %v", rec)
	}
	log.Debugw("execution started", "execution", exe)
	return nil
}

func executionName() (string, error) {
	uid, err := uuid.NewRandom()
	if err != nil {
		return "", errors.Wrap(err, "could not generate a change id")
	}
	return "dnsCertificateWait-" + uid.String(), nil
}
