package main

import (
	"encoding/json"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/codesmith-gmbh/forge/aws/common"
	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	log             = common.MustSugaredLogger()
	stateMachineArn = os.Getenv("STATE_MACHINE_ARN")
)

const (
	CreateAction = route53.ChangeActionUpsert
	DeleteAction = route53.ChangeActionDelete
)

func main() {
	defer common.SyncSugaredLogger(log)
	cfg := common.MustConfig()
	p := &proc{
		acmService: acmService,
		cf:         cloudformation.New(cfg),
		r53:        route53.New(cfg),
		ssm:        ssm.New(cfg),
		step:       sfn.New(cfg),
	}
	lambda.Start(p.processSNSEvent)
}

type proc struct {
	acmService func(properties Properties) (*acm.ACM, error)
	cf         *cloudformation.CloudFormation
	r53        *route53.Route53
	ssm        *ssm.SSM
	step       *sfn.SFN
}

type subproc struct {
	acm  *acm.ACM
	cf   *cloudformation.CloudFormation
	r53  *route53.Route53
	ssm  *ssm.SSM
	step *sfn.SFN
}

// Properties and decoding.

type Properties struct {
	DomainName                                   string
	Region                                       string
	SubjectAlternativeNames                      []string
	Tags                                         []acm.Tag
	HostedZoneName, HostedZoneId, WithCaaRecords string

	withCaaRecords bool `json:"-"`
}

func (p *proc) validateProperties(input map[string]interface{}) (Properties, error) {
	var properties Properties
	if err := mapstructure.Decode(input, &properties); err != nil {
		return properties, err
	}
	if properties.DomainName == "" {
		return properties, errors.New("DomainName is obligatory")
	}
	if properties.WithCaaRecords == "" {
		log.Debugw("defaulting withCaaRecords to true")
		properties.withCaaRecords = true
	} else {
		caa, err := strconv.ParseBool(properties.WithCaaRecords)
		if err != nil {
			return properties, errors.Wrapf(err, "WithCaaRecords must be a boolean: %+v", properties)
		}
		properties.withCaaRecords = caa
	}
	if properties.HostedZoneName == "" && properties.HostedZoneId == "" {
		return properties, errors.Errorf("one of HostedZoneName or HostedZoneId must be defined: %+v", properties)
	}
	if properties.HostedZoneName != "" && properties.HostedZoneId != "" {
		return properties, errors.Errorf("only of HostedZoneName or HostedZoneId may be defined: %+v", properties)
	}
	if err := p.fetchHostedZoneData(&properties); err != nil {
		return properties, err
	}
	if err := checkIsDomainOf(properties.DomainName, properties.HostedZoneName); err != nil {
		return properties, err
	}
	for _, domain := range properties.SubjectAlternativeNames {
		if err := checkIsDomainOf(domain, properties.HostedZoneName); err != nil {
			return properties, err
		}
	}
	return properties, nil
}

func checkIsDomainOf(domain, hostedZoneName string) error {
	if strings.HasSuffix(normalize(domain), normalize(hostedZoneName)) {
		return nil
	}
	return errors.Errorf("%s is not a domain of %s", domain, hostedZoneName)
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
		hs := hostedZone.HostedZones
		if hs == nil || len(hs) == 0 {
			return errors.Errorf("no hosted zones found for hosted zone name: %s", properties.HostedZoneName)
		}
		properties.HostedZoneId = *hostedZone.HostedZones[0].Id
		return nil
	}
}

// Decode and process SNS event

func (p *proc) processSNSEvent(event events.SNSEvent) error {
	for _, rec := range event.Records {
		err := p.processRecord(rec)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *proc) processRecord(record events.SNSEventRecord) error {
	event, err := decodeEvent(record.SNS.Message)
	if err != nil {
		return err
	}

	err = p.processEvent(event, record.SNS.MessageID)

	if err != nil {
		log.Infof("error processing the event, sending failure: %s", err)
		r := cfn.NewResponse(&event)
		r.PhysicalResourceID = event.PhysicalResourceID
		if r.PhysicalResourceID == "" {
			log.Debugf("PhysicalResourceID must exist, copying Log Stream name %s", lambdacontext.LogStreamName)
			r.PhysicalResourceID = lambdacontext.LogStreamName
		}
		r.Status = cfn.StatusFailed
		r.Reason = err.Error()
		log.Infof("sending status failed: %s", r.Reason)
		return r.Send()
	}
	// We do not send a response if the event has been processed: the delete and update (without replacement) events send
	// the success status themselves; the create and update with replacement events start a step functions state machine
	// to wait for the newly created certificate to be issued.
	return nil
}

func decodeEvent(input string) (cfn.Event, error) {
	var event cfn.Event
	log.Debugw("event from sns", "event", input)
	err := json.Unmarshal([]byte(input), &event)
	if err != nil {
		return event, errors.Wrapf(err, "could not unmarshal the data %s", input)
	}
	return event, nil
}

func (p *proc) processEvent(event cfn.Event, snsMessageId string) error {
	properties, err := p.validateProperties(event.ResourceProperties)
	if err != nil {
		return err
	}
	cm, err := p.acmService(properties)
	if err != nil {
		return err
	}
	sp := &subproc{acm: cm, cf: p.cf, step: p.step, ssm: p.ssm, r53: p.r53}
	switch event.RequestType {
	case cfn.RequestDelete:
		return sp.deleteCertificate(event, properties)
	case cfn.RequestCreate:
		return sp.createAndValidateCerificate(event, properties, snsMessageId)
	case cfn.RequestUpdate:
		oldProperties, err := p.validateProperties(event.OldResourceProperties)
		if err != nil {
			return err
		}
		return sp.updateCertificate(event, oldProperties, properties, snsMessageId)
	default:
		_, _, err := common.UnknownRequestType(event)
		return err
	}
}

// Creation

func (p *subproc) createAndValidateCerificate(event cfn.Event, properties Properties, snsMessageId string) error {
	skip, err := p.shouldSkipMessage(event, snsMessageId)
	if err != nil {
		return err
	}
	if skip {
		return nil
	}
	certificateArn, err := p.createCertificateAndTags(properties)
	if err != nil {
		return err
	}
	err = p.createRecordSetGroup(certificateArn, properties)
	if err != nil {
		return err
	}
	return p.startWaitStateMachine(certificateArn, event)
}

// Important: the following code works only because the ReservedConcurrentExecutions of the the DnsCertificate lamdba
// function it set to 1 and so all SNS events are serialized.
func (p *subproc) shouldSkipMessage(event cfn.Event, snsMessageId string) (bool, error) {
	log.Debugw("checking for sns message id",
		"stackID", event.StackID,
		"logicalResourceID", event.LogicalResourceID,
		"snsMessageId", snsMessageId)
	parameterName := common.DnsCertificeSnsMessageIdParameterName(event.StackID, event.LogicalResourceID)
	param, err := p.ssm.GetParameterRequest(&ssm.GetParameterInput{
		Name: &parameterName,
	}).Send()
	if err != nil {
		awsErr, ok := err.(awserr.RequestFailure)
		if !ok || awsErr.StatusCode() != 400 || awsErr.Code() != "ParameterNotFound" {
			return false, errors.Wrapf(err, "could not fetch the parameter %s", parameterName)
		}
	}
	if param != nil && *param.Parameter.Value == snsMessageId {
		log.Infow("already processed", "snsMessageId", snsMessageId)
		return true, nil
	}
	overwrite := true
	_, err = p.ssm.PutParameterRequest(&ssm.PutParameterInput{
		Name:      &parameterName,
		Value:     &snsMessageId,
		Type:      ssm.ParameterTypeString,
		Overwrite: &overwrite,
	}).Send()
	if err != nil {
		return false, errors.Wrapf(err, "can put the parameter %s with the new message ID %s", parameterName, snsMessageId)
	}
	return false, nil
}

func (p *subproc) deleteSnsMessageIdParameter(event cfn.Event) error {
	parameterName := common.DnsCertificeSnsMessageIdParameterName(event.StackID, event.LogicalResourceID)
	_, err := p.ssm.DeleteParameterRequest(&ssm.DeleteParameterInput{
		Name: &parameterName,
	}).Send()
	if err != nil {
		awsErr, ok := err.(awserr.RequestFailure)
		if !ok || awsErr.StatusCode() != 400 || awsErr.Code() != "ParameterNotFound" {
			return errors.Wrapf(err, "could not delete the parameter %s", parameterName)
		}
	}
	return nil
}

func (p *subproc) createCertificateAndTags(properties Properties) (string, error) {
	// 1. Create the certificate with certificate transparency logging enabled
	res, err := p.acm.RequestCertificateRequest(&acm.RequestCertificateInput{
		DomainName:       &properties.DomainName,
		ValidationMethod: acm.ValidationMethodDns,
		Options: &acm.CertificateOptions{
			CertificateTransparencyLoggingPreference: acm.CertificateTransparencyLoggingPreferenceEnabled,
		},
		SubjectAlternativeNames: properties.SubjectAlternativeNames,
	}).Send()
	if err != nil {
		return "", errors.Wrap(err, "could not create the certificate")
	}

	// 2. If applicable, create the tags
	if len(properties.Tags) > 0 {
		_, err = p.acm.AddTagsToCertificateRequest(&acm.AddTagsToCertificateInput{
			CertificateArn: res.CertificateArn,
			Tags:           properties.Tags,
		}).Send()
		if err != nil {
			return *res.CertificateArn, errors.Wrapf(err, "could not add tags to certificate %s", *res.CertificateArn)
		}
	}
	return *res.CertificateArn, err
}

func (p *subproc) startWaitStateMachine(certificateArn string, event cfn.Event) error {
	event.ResourceProperties["CertificateArn"] = certificateArn
	name, err := executionName()
	if err != nil {
		return err
	}
	bytes, err := json.Marshal(event)
	if err != nil {
		return errors.Wrapf(err, "could not marshall the event %v", event)
	}
	msg := string(bytes)
	exe, err := p.step.StartExecutionRequest(&sfn.StartExecutionInput{
		Input:           &msg,
		Name:            &name,
		StateMachineArn: &stateMachineArn,
	}).Send()
	if err != nil {
		return errors.Wrapf(err, "could not start the execution for the event %v", event)
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

// Update

func (p *subproc) updateCertificate(event cfn.Event, old Properties, new Properties, snsMessageId string) error {
	if needsNew(event, old, new) {
		log.Infow("new certificate needed", "StackID", event.StackID, "LogicalResourceID", event.LogicalResourceID)
		log.Debugw("delete old dns records preempively", "old", old)
		err := p.deleteRecordSetGroup(event.PhysicalResourceID, old)
		if err != nil {
			return err
		}
		log.Debugw("create new certificate", "new", new)
		return p.createAndValidateCerificate(event, new, snsMessageId)
	}
	if !tagsSame(old.Tags, new.Tags) {
		err := p.updateTags(event, new)
		if err != nil {
			return err
		}
	}
	if caaRecordsChanged(old, new) {
		var err error
		if new.withCaaRecords {
			err = p.createCaaRecords(event.PhysicalResourceID, new)
		} else {
			err = p.deleteCaaRecords(event.PhysicalResourceID, new)
		}
		if err != nil {
			return err
		}
	}

	return sendSuccess(event)
}

func needsNew(event cfn.Event, old, new Properties) bool {
	return new.DomainName != old.DomainName ||
		!common.IsSameRegion(event, old.Region, new.Region) ||
		!sameSubjectAlternativeNames(new.SubjectAlternativeNames, old.SubjectAlternativeNames) ||
		old.HostedZoneId != new.HostedZoneId
}

func sameSubjectAlternativeNames(san1, san2 []string) bool {
	if san1 == nil && san2 == nil {
		return true
	}
	if san1 == nil || san2 == nil {
		return false
	}
	if len(san1) != len(san2) {
		return false
	}
	var san1Set = make(map[string]bool)
	for _, san := range san1 {
		san1Set[san] = true
	}
	for _, san := range san2 {
		_, ok := san1Set[san]
		if !ok {
			return false
		}
	}
	return true
}

func tagsSame(old, new []acm.Tag) bool {
	if old == nil && new == nil {
		return true
	}
	if old == nil || new == nil {
		return false
	}
	if len(old) != len(new) {
		return false
	}
	var tagMap = make(map[string]string)
	for _, tag := range old {
		tagMap[*tag.Key] = *tag.Value
	}
	for _, tag := range new {
		val := tagMap[*tag.Key]
		if val != *tag.Value {
			return false
		}
	}
	return true
}

func caaRecordsChanged(old Properties, new Properties) bool {
	return old.withCaaRecords && !new.withCaaRecords || !old.withCaaRecords && new.withCaaRecords
}

// Updating is quite straightforward: we delete all the tags before
// recreating them. We must gather the CNAME records to send as attribute
// to the response.
func (p *subproc) updateTags(event cfn.Event, properties Properties) error {
	// 1. we first fetch the tags.
	tags, err := p.acm.ListTagsForCertificateRequest(&acm.ListTagsForCertificateInput{
		CertificateArn: &event.PhysicalResourceID,
	}).Send()
	if err != nil {
		return errors.Wrapf(err, "could not list tags for certificate %s", event.PhysicalResourceID)
	}
	// 2. we remove them all if there are any
	if areDefined(tags.Tags) {
		_, err = p.acm.RemoveTagsFromCertificateRequest(&acm.RemoveTagsFromCertificateInput{
			CertificateArn: &event.PhysicalResourceID,
			Tags:           tags.Tags,
		}).Send()
		if err != nil {
			return errors.Wrapf(err, "could not remove tags for certificate %s", event.PhysicalResourceID)
		}
	}
	// 3. we create the new tags if they are any
	if areDefined(properties.Tags) {
		_, err = p.acm.AddTagsToCertificateRequest(&acm.AddTagsToCertificateInput{
			CertificateArn: &event.PhysicalResourceID,
			Tags:           properties.Tags,
		}).Send()
		if err != nil {
			return errors.Wrapf(err, "could not add tags for certificate %s", event.PhysicalResourceID)
		}
	}
	return nil
}

func areDefined(tags []acm.Tag) bool {
	return tags != nil && len(tags) > 0
}

// Deletion

func (p *subproc) deleteCertificate(event cfn.Event, properties Properties) error {
	if common.IsCertificateArn(event.PhysicalResourceID) {
		// we always delete the dns records before creating a new certificates if an update requires a replacement
		beingReplaced, err := p.isBeingReplaced(event)
		if err != nil {
			return err
		}
		if !beingReplaced {
			err = p.deleteRecordSetGroup(event.PhysicalResourceID, properties)
			if err != nil {
				return err
			}
			err = p.deleteSnsMessageIdParameter(event)
			if err != nil {
				return err
			}
		}
		_, err = p.acm.DeleteCertificateRequest(&acm.DeleteCertificateInput{
			CertificateArn: &event.PhysicalResourceID,
		}).Send()
		if err != nil {
			return errors.Wrapf(err, "could not delete the certificate %s", event.PhysicalResourceID)
		}
	}
	return sendSuccess(event)
}

// If the physical id of a resource being deleted is different from the physical id of the resource with the same
// logical id in the stack, then we have a replacement; otherwise, we have a simple deletion.
func (p *subproc) isBeingReplaced(event cfn.Event) (bool, error) {
	res, err := p.cf.DescribeStackResourceRequest(&cloudformation.DescribeStackResourceInput{
		StackName:         &event.StackID,
		LogicalResourceId: &event.LogicalResourceID,
	}).Send()
	if err != nil {
		return false, errors.Wrapf(err, "could not describe the resource %s on the stack %s", event.StackID, event.LogicalResourceID)
	}
	return *res.StackResourceDetail.PhysicalResourceId != event.PhysicalResourceID, nil
}

// DNS records

func (p *subproc) createCaaRecords(certificateArn string, properties Properties) error {
	changes, err := p.generateChanges(certificateArn, properties, CreateAction, caaSpec)
	if err != nil {
		return err
	}
	err = p.executeBatchChangeRequest(properties.HostedZoneId, changes)
	return err
}

func (p *subproc) deleteCaaRecords(certificateArn string, properties Properties) error {
	changes, err := p.generateChanges(certificateArn, properties, DeleteAction, caaSpec)
	if err != nil {
		return err
	}
	return p.deleteChanges(properties.HostedZoneId, changes)
}

func (p *subproc) deleteRecordSetGroup(certificateArn string, properties Properties) error {
	changes, err := p.generateChanges(certificateArn, properties, DeleteAction, validationSpec(properties))
	if err != nil {
		return err
	}
	return p.deleteChanges(properties.HostedZoneId, changes)
}

func (p *subproc) deleteChanges(hostedZoneId string, changes []route53.Change) error {
	// we try first to delete batch by batch
	err := p.executeBatchChangeRequest(hostedZoneId, changes)
	if err != nil {
		// if not possible, we delete record per record with tolerance if a record has been deleted manually
		// already.
		log.Debugw("could not delete the records in batch, deleting one by one", "err", err)
		err = p.executeDeleteRequests(hostedZoneId, changes)
	}
	return err
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
		withCaa:           properties.withCaaRecords,
	}
}

func (p *subproc) createRecordSetGroup(certificateArn string, properties Properties) error {
	changes, err := p.generateChanges(certificateArn, properties, CreateAction, validationSpec(properties))
	if err != nil {
		return err
	}
	log.Debugw("changes for creation", "changes", changes)
	return p.executeBatchChangeRequest(properties.HostedZoneId, changes)
}

func (p *subproc) generateChanges(certificateArn string, properties Properties, changeAction route53.ChangeAction, spec generationSpec) ([]route53.Change, error) {
	cert, err := p.describeCertificate(certificateArn, properties)
	if err != nil {
		return nil, err
	}

	changes := make([]route53.Change, 0, len(cert.DomainValidationOptions)*2)
	for _, opt := range cert.DomainValidationOptions {
		log.Debugw("validation options", "domainName", *opt.DomainName, "hostedZoneName", properties.HostedZoneName)
		if spec.withDnsValidation {
			changes = append(changes, dnsValidationChange(opt, changeAction))
		}
		if spec.withCaa {
			changes = append(changes, caaChange(opt, changeAction))
		}
	}
	return changes, nil
}

// Waiting for the data for the CNAME records requires a loop and waiting
// since those are created by AWS asynchronously and added to the
// certificate information only when they have been properly created. We
// wait at most 3 minutes with 3 seconds interval.
func (p *subproc) describeCertificate(certificateArn string, properties Properties) (*acm.CertificateDetail, error) {
OUTER:
	for i := 0; i < 60; i++ {
		cert, err := p.acm.DescribeCertificateRequest(&acm.DescribeCertificateInput{
			CertificateArn: &certificateArn,
		}).Send()
		if err != nil {
			return nil, errors.Wrapf(err, "could not fetch certificate %s", certificateArn)
		}
		log.Debugw("describe certificate", "i", i, "cert", cert)
		options := cert.Certificate.DomainValidationOptions
		if options == nil {
			log.Debugw("no options")
		} else {
			if len(options) == len(properties.SubjectAlternativeNames)+1 {
				for _, option := range options {
					if option.ResourceRecord == nil {
						log.Debugw("no resource record", "option", option)
						time.Sleep(3 * time.Second)
						continue OUTER
					}
				}
				return cert.Certificate, nil
			} else {
				log.Debugw("different lengths", "len_options", len(options), "len_san_plus", len(properties.SubjectAlternativeNames)+1)
			}
		}

		time.Sleep(3 * time.Second)
	}
	return nil, errors.Errorf("no DNS entries for certificate %s", certificateArn)
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

func (p *subproc) executeBatchChangeRequest(hostedZoneId string, changes []route53.Change) error {
	changeInfo, err := p.r53.ChangeResourceRecordSetsRequest(&route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{
			Changes: changes,
			Comment: &comment,
		},
		HostedZoneId: &hostedZoneId,
	}).Send()
	if err != nil {
		return errors.Wrap(err, "could not execute record set change batch")
	}
	return p.waitForChange(changeInfo.ChangeInfo)
}

func (p *subproc) executeDeleteRequests(hostedZoneId string, changes []route53.Change) error {
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

func (p *subproc) waitForChange(changeInfo *route53.ChangeInfo) error {
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

// Utilities

func acmService(properties Properties) (*acm.ACM, error) {
	var cfg aws.Config
	var err error
	if len(properties.Region) > 0 {
		cfg, err = external.LoadDefaultAWSConfig(external.WithRegion(properties.Region))
		if err != nil {
			return nil, errors.Wrapf(err, "could not load config with region %s", properties.Region)
		}
	} else {
		cfg, err = external.LoadDefaultAWSConfig()
		if err != nil {
			return nil, errors.Wrap(err, "could not load default config")
		}
	}
	return acm.New(cfg), nil
}

func sendSuccess(event cfn.Event) error {
	r := cfn.NewResponse(&event)
	r.PhysicalResourceID = event.PhysicalResourceID
	r.Status = cfn.StatusSuccess
	return r.Send()
}
