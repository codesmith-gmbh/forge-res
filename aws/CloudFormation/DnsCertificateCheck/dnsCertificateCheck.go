package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/codesmith-gmbh/forge/aws/common"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"regexp"
)

const MaxRoundCount = 60

var log = common.MustSugaredLogger()

type proc struct {
	acmService func(certificateArn string) (*acm.ACM, error)
}

func main() {
	defer common.SyncSugaredLogger(log)
	p := &proc{acmService: acmService}
	lambda.Start(p.processEvent)
}

func decodeEvent(eventRaw map[string]interface{}) (cfn.Event, error) {
	var event cfn.Event
	err := mapstructure.Decode(eventRaw, &event)
	return event, err
}

func (p *proc) processEvent(ctx context.Context, eventRaw map[string]interface{}) (map[string]interface{}, error) {
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
	check, err := p.checkCertificate(ctx, event, round)
	if err != nil {
		return eventRaw, err
	}
	eventRaw["IsCertificateIssued"] = check
	eventRaw["Round"] = round + 1
	return eventRaw, nil
}

func (p *proc) checkCertificate(ctx context.Context, event cfn.Event, round int) (bool, error) {
	certificateArn, ok := event.ResourceProperties["CertificateArn"].(string)
	if !ok {
		return false, errors.Errorf("the event does not contain sufficient information, the certificate arn is missing")
	}
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
	}).Send(ctx)
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

// ### SDK client
//
// We use the
// [ACM sdk v2](https://github.com/aws/aws-sdk-go-v2/tree/master/service/acm)
// to create the certificate. The client is created with the default
// credential chain loader and with the region of the certificate
func acmService(certificateArn string) (*acm.ACM, error) {
	var cfg aws.Config
	region, err := certificateRegion(certificateArn)
	if err != nil {
		return nil, err
	}

	cfg, err = external.LoadDefaultAWSConfig(external.WithRegion(region))
	if err != nil {
		return nil, errors.Wrapf(err, "could not load config with region %s", region)
	}

	return acm.New(cfg), nil
}

var certificateRegionRegExp = regexp.MustCompile("^arn:aws.*:acm:(.+?):")

func certificateRegion(arn string) (string, error) {
	matches := certificateRegionRegExp.FindStringSubmatch(arn)
	if len(matches) == 2 {
		return matches[1], nil
	}
	return "", errors.Errorf("could not extract the region from the arn %s", arn)
}
