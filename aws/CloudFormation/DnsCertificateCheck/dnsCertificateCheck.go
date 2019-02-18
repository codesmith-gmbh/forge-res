package main

import (
	"fmt"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	forgeacm "github.com/codesmith-gmbh/forge/aws/acm"
	"github.com/codesmith-gmbh/forge/aws/common"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

const MaxRoundCount = 60

var log = common.MustSugaredLogger()

type proc struct {
	acmService func(certificateArn string) (*acm.ACM, error)
}

func main() {
	defer common.SyncSugaredLogger(log)
	p := &proc{acmService: forgeacm.AcmService}
	lambda.Start(p.processEvent)
}

func decodeEvent(eventRaw map[string]interface{}) (cfn.Event, error) {
	var event cfn.Event
	err := mapstructure.Decode(eventRaw, &event)
	return event, err
}

func (p *proc) processEvent(eventRaw map[string]interface{}) (map[string]interface{}, error) {
	log.Debugw("event", "event", eventRaw)
	event, err := decodeEvent(eventRaw)
	if err != nil {
		return nil, errors.Wrapf(err, "did not receive a cloudformation event %v", eventRaw)
	}
	roundF, ok := eventRaw["Round"].(float64)
	var round int
	if ok {
		round = int(roundF)
	} else {
		round = 0
	}
	check, err := p.checkCertificate(event, round)
	eventRaw["IsCertificateIssued"] = check
	eventRaw["Round"] = round + 1
	return eventRaw, nil
}

func (p *proc) checkCertificate(event cfn.Event, round int) (bool, error) {
	certificateArn := event.ResourceProperties["CertificateArn"].(string)
	response := cfn.NewResponse(&event)
	response.PhysicalResourceID = certificateArn
	if event.RequestType == cfn.RequestDelete {
		response.Status = cfn.StatusSuccess
		err := response.Send()
		return true, err
	}
	if round >= MaxRoundCount {
		response.Status = cfn.StatusFailed
		response.Reason = fmt.Sprintf("certificate %s did not stabilise", certificateArn)
		err := response.Send()
		return true, err
	}
	cm, err := p.acmService(certificateArn)
	if err != nil {
		response.Status = cfn.StatusFailed
		response.Reason = fmt.Sprintf("Could not create acm client for the certificate: %s", certificateArn)
		err := response.Send()
		return true, err
	}
	cert, err := cm.DescribeCertificateRequest(&acm.DescribeCertificateInput{
		CertificateArn: &certificateArn,
	}).Send()
	if err != nil {
		response.Status = cfn.StatusFailed
		response.Reason = fmt.Sprintf("Could not describe for the certificate: %s", certificateArn)
		err := response.Send()
		return true, err
	}
	status := cert.Certificate.Status
	switch status {
	case acm.CertificateStatusIssued:
		response.Status = cfn.StatusSuccess
		err := response.Send()
		return true, err
	case acm.CertificateStatusPendingValidation:
		return false, nil
	default:
		response.Status = cfn.StatusFailed
		response.Reason = fmt.Sprintf("the certificate %s is in invalid status %s", certificateArn, status)
		err := response.Send()
		return true, err
	}
}
