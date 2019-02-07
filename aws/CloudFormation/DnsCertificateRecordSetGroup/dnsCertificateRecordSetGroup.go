package main

import (
	"context"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/codesmith-gmbh/forge/aws/common"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"regexp"
	"strings"
	"time"
)

var log = common.MustSugaredLogger()

// The lambda is started using the AWS lambda go sdk. The handler function
// does the actual work of creating the certificate. Cloudformation sends
// an event to signify that a resources must be created, updated or
// deleted.
func main() {
	defer common.SyncSugaredLogger(log)
	cfg := common.MustConfig()
	r53 := route53.New(cfg)
	p := &proc{r53: r53, acmService: acmService}
	lambda.Start(cfn.LambdaWrap(p.processEvent))
}

type proc struct {
	r53        *route53.Route53
	acmService func(properties Properties) (*acm.ACM, error)
}

type Properties struct {
	CertificateArn, HostedZoneName, HostedZoneId string
	WithCaaRecords                               bool
}

func (p *proc) decodeProperties(input map[string]interface{}) (Properties, error) {
	var properties Properties
	if err := mapstructure.Decode(input, &properties); err != nil {
		return properties, err
	}
	if !common.IsCertificateArn(properties.CertificateArn) {
		return properties, errors.Errorf("CertificateArn must be defined and be a ARN for a certificate: %v", properties)
	}
	if properties.HostedZoneName == "" && properties.HostedZoneId == "" {
		return properties, errors.Errorf("one of HostedZoneName or HostedZoneId must be defined: %v", properties)
	}
	if properties.HostedZoneName != "" && properties.HostedZoneId != "" {
		return properties, errors.Errorf("only of HostedZoneName or HostedZoneId may be defined: %v", properties)
	}
	if err := p.fetchHostedZoneData(&properties); err != nil {
		return properties, err
	}
	return properties, nil
}

func (p *proc) fetchHostedZoneData(properties *Properties) error {
	if properties.HostedZoneName == "" {
		hostedZone, err := p.r53.GetHostedZoneRequest(&route53.GetHostedZoneInput{
			Id: &properties.HostedZoneId,
		}).Send()
		if err != nil {
			return errors.Wrapf(err, "could not fetch hosted zone name for hosted zone id: %s", properties.HostedZoneId)
		}
		properties.HostedZoneName = *hostedZone.HostedZone.Name
		return nil
	} else {
		hostedZone, err := p.r53.ListHostedZonesByNameRequest(&route53.ListHostedZonesByNameInput{
			DNSName: &properties.HostedZoneName,
		}).Send()
		if err != nil {
			return errors.Wrapf(err, "could not fetch hosted zone id for hosted zone name: %s", properties.HostedZoneName)
		}
		properties.HostedZoneId = *hostedZone.HostedZones[0].Id
		return nil
	}
}

func (p *proc) processEvent(_ context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	properties, err := p.decodeProperties(event.ResourceProperties)
	if err != nil {
		return "", nil, err
	}
	switch event.RequestType {
	case cfn.RequestDelete:
		err := p.deleteRecordSetGroup(properties)
		if err != nil {
			return event.PhysicalResourceID, nil, err
		}
		return event.PhysicalResourceID, nil, nil
	case cfn.RequestCreate:
		return p.createRecordSetGroup(properties)
	case cfn.RequestUpdate:
		oldProperties, err := p.decodeProperties(event.OldResourceProperties)
		if err != nil {
			return event.PhysicalResourceID, nil, err
		}
		if updatable(oldProperties, properties) {
			var err error
			if properties.WithCaaRecords {
				err = p.createCaaRecords(properties)
			} else {
				err = p.deleteCaaRecords(properties)
			}
			return event.PhysicalResourceID, nil, err
		} else {
			return p.createRecordSetGroup(properties)
		}
	default:
		return common.UnknownRequestType(event)
	}
}

func (p *proc) createRecordSetGroup(properties Properties) (string, map[string]interface{}, error) {
	changes, err := p.generateChanges(properties, route53.ChangeActionCreate, validationSpec(properties))
	if err != nil {
		return "", nil, err
	}
	log.Debugw("changes for creation", "changes", changes)
	changeId, err := p.executeBatchChangeRequest(properties.HostedZoneId, changes)
	return changeId, nil, err
}

func (p *proc) deleteRecordSetGroup(properties Properties) error {
	changes, err := p.generateChanges(properties, route53.ChangeActionDelete, validationSpec(properties))
	if err != nil {
		return err
	}
	return p.deleteChanges(properties.HostedZoneId, changes)
}

func (p *proc) deleteChanges(hostedZoneId string, changes []route53.Change) error {
	// we try first to delete batch by batch
	_, err := p.executeBatchChangeRequest(hostedZoneId, changes)
	if err != nil {
		// if not possible, we delete record per record with tolerance if a record has been deleted manually
		// already.
		log.Debugw("could not delete the records in batch, deleting one by one", "err", err)
		err = p.executeDeleteRequests(hostedZoneId, changes)
	}
	return err
}

func updatable(old Properties, new Properties) bool {
	return old.HostedZoneId == new.HostedZoneId &&
		old.HostedZoneName == new.HostedZoneName &&
		old.CertificateArn == new.CertificateArn &&
		old.WithCaaRecords != new.WithCaaRecords
}

func (p *proc) createCaaRecords(properties Properties) error {
	changes, err := p.generateChanges(properties, route53.ChangeActionCreate, caaSpec)
	if err != nil {
		return err
	}
	_, err = p.executeBatchChangeRequest(properties.HostedZoneId, changes)
	return err
}

func (p *proc) deleteCaaRecords(properties Properties) error {
	changes, err := p.generateChanges(properties, route53.ChangeActionDelete, caaSpec)
	if err != nil {
		return err
	}
	return p.deleteChanges(properties.HostedZoneId, changes)
}

type generationSpec struct {
	withDnsValidation, withCaa bool
}

var caaSpec = generationSpec{
	withDnsValidation: false,
	withCaa:           true,
}

func validationSpec(properties Properties) generationSpec {
	return generationSpec{
		withDnsValidation: true,
		withCaa:           properties.WithCaaRecords,
	}
}

func (p *proc) generateChanges(properties Properties, changeAction route53.ChangeAction, spec generationSpec) ([]route53.Change, error) {
	acms, err := p.acmService(properties)
	if err != nil {
		return nil, err
	}
	cert, err := acms.DescribeCertificateRequest(&acm.DescribeCertificateInput{
		CertificateArn: &properties.CertificateArn,
	}).Send()
	if err != nil {
		return nil, errors.Wrapf(err, "could not describe certificate %s", properties.CertificateArn)
	}

	changes := make([]route53.Change, 0, len(cert.Certificate.DomainValidationOptions)*2)
	for _, opt := range cert.Certificate.DomainValidationOptions {
		log.Debugw("validation options", "domainName", *opt.DomainName, "hostedZoneName", properties.HostedZoneName)
		if strings.HasSuffix(normalize(*opt.DomainName), properties.HostedZoneName) {
			if spec.withDnsValidation {
				changes = append(changes, dnsValidationChange(opt, changeAction))
			}
			if spec.withCaa {
				changes = append(changes, caaChange(opt, changeAction))
			}
		}
	}
	return changes, nil
}

func normalize(domainName string) string {
	if strings.HasSuffix(domainName, ".") {
		return domainName
	} else {
		return domainName + "."
	}
}

var ttl int64 = 300

func dnsValidationChange(validation acm.DomainValidation, changeAction route53.ChangeAction) route53.Change {
	return route53.Change{
		Action: changeAction,
		ResourceRecordSet: &route53.ResourceRecordSet{
			Name: validation.ResourceRecord.Name,
			ResourceRecords: []route53.ResourceRecord{{
				Value: validation.ResourceRecord.Value,
			}},
			Type: route53.RRType(validation.ResourceRecord.Type),
			TTL:  &ttl,
		},
	}
}

var caaRecord = "0 issue \"amazon.com\""

func caaChange(validation acm.DomainValidation, changeAction route53.ChangeAction) route53.Change {
	caaName := *validation.DomainName + "."
	return route53.Change{
		Action: changeAction,
		ResourceRecordSet: &route53.ResourceRecordSet{
			Name: &caaName,
			ResourceRecords: []route53.ResourceRecord{{
				Value: &caaRecord,
			}},
			Type: route53.RRTypeCaa,
			TTL:  &ttl,
		},
	}
}

var comment = "by Codesmith Forge DnsCertificateRecordSetGroup custom resource"

func (p *proc) executeBatchChangeRequest(hostedZoneId string, changes []route53.Change) (string, error) {
	changeInfo, err := p.r53.ChangeResourceRecordSetsRequest(&route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{
			Changes: changes,
			Comment: &comment,
		},
		HostedZoneId: &hostedZoneId,
	}).Send()
	if err != nil {
		return "", errors.Wrap(err, "could not execute record set change batch")
	}
	return *changeInfo.ChangeInfo.Id, p.waitForChange(changeInfo.ChangeInfo)
}

func (p *proc) executeDeleteRequests(hostedZoneId string, changes []route53.Change) error {
	for _, change := range changes {
		changeInfo, err := p.r53.ChangeResourceRecordSetsRequest(&route53.ChangeResourceRecordSetsInput{
			ChangeBatch: &route53.ChangeBatch{
				Changes: []route53.Change{change},
				Comment: &comment,
			},
			HostedZoneId: &hostedZoneId,
		}).Send()
		if err != nil {
			msg := err.Error()
			if strings.Contains(msg, "[Tried to delete resource record set") {
				continue
			} else {
				return errors.Wrap(err, "could not execute delete record set change")
			}
		}
		err = p.waitForChange(changeInfo.ChangeInfo)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *proc) waitForChange(changeInfo *route53.ChangeInfo) error {
	changeId := *changeInfo.Id
	changeStatus := changeInfo.Status
	for i := 0; i < 60; i++ {
		if changeStatus == route53.ChangeStatusInsync {
			return nil
		}
		time.Sleep(3 * time.Second)
		res, err := p.r53.GetChangeRequest(&route53.GetChangeInput{
			Id: &changeId,
		}).Send()
		if err != nil {
			return errors.Wrapf(err, "could not fetch the change %s", changeId)
		}
		changeStatus = res.ChangeInfo.Status
	}
	return errors.Errorf("change %s did not sync in time", changeId)
}

// ### SDK client
//
// We use the
// [ACM sdk v2](https://github.com/aws/aws-sdk-go-v2/tree/master/service/acm)
// to create the certificate. The client is created with the default
// credential chain loader, if need be with the supplied region.
func acmService(properties Properties) (*acm.ACM, error) {
	var cfg aws.Config
	region, err := certificateRegion(properties.CertificateArn)
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
