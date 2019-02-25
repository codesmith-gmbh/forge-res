package main

import (
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/codesmith-gmbh/forge/aws/testCommon"
	"github.com/pkg/errors"
	"strings"
	"testing"
)

func TestNeedsNew(t *testing.T) {
	// 1. correctness
	for i, test := range []struct{ prop1, prop2 Properties }{
		// test 0
		{Properties{
			DomainName:              "test.com",
			Region:                  "us-east-1",
			SubjectAlternativeNames: []string{"hello1.test.com", "hello2.test.com"},
			Tags:                    []acm.Tag{tag("key", "value1")},
		}, Properties{
			DomainName:              "hello3.test.com",
			Region:                  "us-east-1",
			SubjectAlternativeNames: []string{"hello2.test.com", "hello1.test.com"},
			Tags:                    []acm.Tag{tag("key", "value2")},
		}},
		// test 1
		{Properties{
			DomainName:              "test.com",
			Region:                  "us-east-1",
			SubjectAlternativeNames: []string{"hello1.test.com", "hello2.test.com"},
			Tags:                    []acm.Tag{tag("key", "value1")},
		}, Properties{
			DomainName:              "test.com",
			Region:                  "eu-west-1",
			SubjectAlternativeNames: []string{"hello2.test.com", "hello1.test.com"},
			Tags:                    []acm.Tag{tag("key", "value2")},
		}},
		// test 2
		{Properties{
			DomainName:              "test.com",
			Region:                  "us-east-1",
			SubjectAlternativeNames: []string{"hello1.test.com", "hello3.test.com"},
			Tags:                    []acm.Tag{tag("key", "value1")},
		}, Properties{
			DomainName:              "test.com",
			Region:                  "us-east-1",
			SubjectAlternativeNames: []string{"hello2.test.com", "hello1.test.com"},
			Tags:                    []acm.Tag{tag("key", "value2")},
		}},
		// test 3
		{Properties{
			DomainName:              "test.com",
			Region:                  "us-east-1",
			SubjectAlternativeNames: []string{"hello1.test.com", "hello2.test.com", "hello3.test.com"},
			Tags:                    []acm.Tag{tag("key", "value1")},
		}, Properties{
			DomainName:              "test.com",
			Region:                  "us-east-1",
			SubjectAlternativeNames: []string{"hello2.test.com", "hello1.test.com"},
			Tags:                    []acm.Tag{tag("key", "value2")},
		}},
	} {
		if !needsNew(cfn.Event{}, test.prop1, test.prop2) {
			t.Errorf("TestOnlyTagsChangedCompleteness %d, %+v, %+v", i, test.prop1, test.prop2)
		}
	}
	// 2. completeness
	if needsNew(
		cfn.Event{},
		Properties{
			DomainName:              "test.com",
			Region:                  "us-east-1",
			SubjectAlternativeNames: []string{"hello1.test.com", "hello2.test.com"},
			Tags:                    []acm.Tag{tag("key", "value1")},
		},
		Properties{
			DomainName:              "test.com",
			Region:                  "us-east-1",
			SubjectAlternativeNames: []string{"hello2.test.com", "hello1.test.com"},
			Tags:                    []acm.Tag{tag("key", "value2")},
		}) {
		t.Error("TestOnlyTagsChangedCorrectness")
	}
}

func TestValidateProperties(t *testing.T) {
	p := mustTestProc()
	// 1. correctness
	for i, input := range []map[string]interface{}{
		{
			"HostedZoneName": "codesmith.ch",
			"DomainName":     "codesmith.ch",
		},
		{
			"HostedZoneName": "codesmith.ch.",
			"DomainName":     "test.codesmith.ch",
			"SubjectAlternativeNames": []string{
				"test2.codesmith.ch",
				"test3.codesmith.ch",
			},
			"WithCaaRecords": "false",
			"Region":         "us-east-1",
			"Tags": []map[string]string{{
				"Key":   "Stan",
				"Value": "stan",
			}},
		},
	} {
		_, err := p.validateProperties(input)
		if err != nil {
			t.Errorf("input %d should validate: %v", i, err)
		}
	}
	// 2. completeness
	for i, input := range []map[string]interface{}{
		{},
		{
			"HostedZoneName": "codesmith.ch",
		},
		{
			"DomainName":     "error.ch",
			"HostedZoneName": "error.ch",
		},
		{
			"DomainName":   "error.ch",
			"HostedZoneId": "???",
		},
		{
			"DomainName": "codesmith.ch",
		},
		{
			"HostedZoneName": "codesmith.ch.",
			"HostedZoneId":   "????",
			"DomainName":     "test.codesmith.ch",
		},
		{
			"HostedZoneName": "codesmith.ch.",
			"DomainName":     "test.error.ch",
		},
		{
			"HostedZoneName": "codesmith.ch.",
			"DomainName":     "test.codesmith.ch",
			"SubjectAlternativeNames": []string{
				"test2.codesmith.ch",
				"test.error.ch",
			},
		},
		{
			"HostedZoneName": "codesmith.ch.",
			"DomainName":     "test.codesmith.ch",
			"WithCaaRecords": "hello",
		},
	} {
		properties, err := p.validateProperties(input)
		if err == nil {
			t.Errorf("input %d should not validate: %+v", i, properties)
		}
	}

}

var messageIdParameterEvent = cfn.Event{StackID: "test", LogicalResourceID: "Certificate"}

func TestIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	p := mustTestProc()
	sp := p.mustSuproc()
	certArn, err := ensureTestCertificateArn(sp, p.mustProperties(t))
	if err != nil {
		t.Fatal(err, "no certificate could be created")
	}
	defer sp.deleteTestCertificate(t, certArn)
	p.certArn = certArn
	t.Run("Creation Change Generation CAA", p.TestCreationChangeGenerationWithCaa)
	t.Run("Creation Change Generation No CAA", p.TestCreationChangeGenerationWithoutCaa)
	t.Run("Update Change Generation CAA", p.TestUpdateChangeGenerationCaa)
	t.Run("Creation/Deletion Standard", p.TestCreationDeletionStandard)
	t.Run("Creation/Deletion Failover", p.TestCreationDeletionFailover)
	t.Run("Faulty Resource Deletion", p.TestFaultyResourceDeletion)
}

func (p *testProc) TestCreationChangeGenerationWithCaa(t *testing.T) {
	sp := p.mustSuproc()
	properties := p.mustProperties(t)
	properties.withCaaRecords = true
	changes, err := sp.generateChanges(p.certArn, properties, CreateAction, validationSpec(properties))
	if err != nil {
		t.Fatal(err, "no changes generated")
	}
	if len(changes) != 4 {
		t.Errorf("4 changes expected, got %v", changes)
	}
	for _, domain := range []string{"test-forge.codesmith.ch.", "test-forge-san.codesmith.ch."} {
		if !hasCnameRecordFor(changes, domain) {
			t.Errorf("no validation CNAME record for %s", domain)
		}
		if !hasCaaRecordFor(changes, domain) {
			t.Errorf("no CAA record for %s", domain)
		}
	}
}

func (p *testProc) TestCreationChangeGenerationWithoutCaa(t *testing.T) {
	sp := p.mustSuproc()
	properties := p.mustProperties(t)
	properties.withCaaRecords = false
	changes, err := sp.generateChanges(p.certArn, properties, CreateAction, validationSpec(properties))
	if err != nil {
		t.Fatal(err, "no changes generated")
	}
	if len(changes) != 2 {
		t.Errorf("2 changes expected, got %v", changes)
	}
	for _, domain := range []string{"test-forge.codesmith.ch.", "test-forge-san.codesmith.ch."} {
		if !hasCnameRecordFor(changes, domain) {
			t.Errorf("no validation CNAME record for %s", domain)
		}
		if hasCaaRecordFor(changes, domain) {
			t.Errorf("CAA record for %s", domain)
		}
	}
}

func (p *testProc) TestUpdateChangeGenerationCaa(t *testing.T) {
	sp := p.mustSuproc()
	properties := p.mustProperties(t)
	properties.WithCaaRecords = "false"
	changes, err := sp.generateChanges(p.certArn, properties, CreateAction, caaSpec)
	if err != nil {
		t.Fatal(err, "no changes generated")
	}
	if len(changes) != 2 {
		t.Errorf("2 changes expected, got %v", changes)
	}
	for _, domain := range []string{"test-forge.codesmith.ch.", "test-forge-san.codesmith.ch."} {
		if hasCnameRecordFor(changes, domain) {
			t.Errorf("validation CNAME record for %s", domain)
		}
		if !hasCaaRecordFor(changes, domain) {
			t.Errorf("no CAA record for %s", domain)
		}
	}
}

func (p *testProc) TestCreationDeletionStandard(t *testing.T) {
	sp := p.mustSuproc()
	properties := p.mustProperties(t)
	err := sp.createRecordSetGroup(p.certArn, properties)
	if err != nil {
		t.Fatalf("could not create the record set group: %v", err)
	}
	defer sp.tearDownCreation(t, p.certArn, properties)
}

func (p *testProc) TestCreationDeletionFailover(t *testing.T) {
	sp := p.mustSuproc()
	properties := p.mustProperties(t)
	err := sp.createRecordSetGroup(p.certArn, properties)
	if err != nil {
		t.Fatalf("could not create the record set group: %v", err)
	}
	defer sp.tearDownCreation(t, p.certArn, properties)
	err = sp.deleteOneCaaRecord(p.certArn, properties)
	if err != nil {
		t.Errorf("could not manually delete a CAA record")
	}
}

func (p *testProc) TestFaultyResourceDeletion(t *testing.T) {
	properties := p.mustProperties(t)
	sp := p.mustSuproc()
	err := sp.deleteRecordSetGroup(p.certArn, properties)
	if err != nil {
		t.Errorf("could not deleting faulty resources")
	}
}

func TestExtractStackId(t *testing.T) {
	// 1. correctness
	for i, test := range []struct{ arn, id string }{
		{arn: "arn:aws:cloudformation:eu-west-1:1234567890:stack/Test/1f2fb450-3767-11e9-a02b-0a9391483dc6", id: "Test/1f2fb450-3767-11e9-a02b-0a9391483dc6"},
	} {
		id, err := extractStackId(test.arn)
		if err != nil {
			t.Errorf("case %d with error %s", i, err)
		}
		if test.id != id {
			t.Errorf("case %d, expecting %s got %s", i, test.id, id)
		}
	}
	// 2. completeness
	for i, test := range []string{
		"",
		"Test/1f2fb450-3767-11e9-a02b-0a9391483dc6",
		"arn:aws:iam::123456789012:user/TestUser",
	} {
		_, err := extractStackId(test)
		if err == nil {
			t.Errorf("case %d without error", i)
		}
	}
}

func TestSnsMessageIdParameter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	p := mustTestProc()
	sp := p.mustSuproc()
	defer sp.mustDeleteSnsMessageIdParameter(messageIdParameterEvent)
	err := sp.deleteSnsMessageIdParameter(messageIdParameterEvent)
	if err != nil {
		t.Error(err, "delete should not fail on inexisting parameter")
	}
	skip, err := sp.shouldSkipMessage(messageIdParameterEvent, "1")
	if err != nil {
		t.Error(err)
	}
	if skip {
		t.Error("should not skip for message 1")
	}
	skip, err = sp.shouldSkipMessage(messageIdParameterEvent, "1")
	if err != nil {
		t.Error(err)
	}
	if !skip {
		t.Error("should skip for message 1")
	}
	skip, err = sp.shouldSkipMessage(messageIdParameterEvent, "2")
	if err != nil {
		t.Error(err)
	}
	if skip {
		t.Error("should skip for message 2")
	}
}

func (p *subproc) mustDeleteSnsMessageIdParameter(event cfn.Event) {
	err := p.deleteSnsMessageIdParameter(messageIdParameterEvent)
	if err != nil {
		panic(err)
	}
}

// Helper functions and predicates.

func tag(key string, val string) acm.Tag {
	return acm.Tag{Key: &key, Value: &val}
}

type testProc struct {
	proc
	cfg     aws.Config
	certArn string
}

func mustTestProc() *testProc {
	cfg := testCommon.MustTestConfig()
	cm := acm.New(cfg)
	acmService := func(_ Properties) (*acm.ACM, error) {
		return cm, nil
	}
	return &testProc{proc: proc{r53: route53.New(cfg), acmService: acmService, ssm: ssm.New(cfg)}, cfg: cfg}
}

func (p *testProc) mustProperties(t *testing.T) Properties {
	properties := Properties{
		HostedZoneName:          "codesmith.ch.",
		DomainName:              "test-forge.codesmith.ch",
		SubjectAlternativeNames: []string{"test-forge-san.codesmith.ch"},
	}
	err := p.fetchHostedZoneData(&properties)
	if err != nil {
		t.Fatal(err, "could not fetch hosted zone data")
	}
	return properties
}

func (p *testProc) mustSuproc() *subproc {
	cm, err := p.acmService(Properties{})
	if err != nil {
		panic(err)
	}
	return &subproc{acm: cm, cf: p.cf, step: p.step, ssm: p.ssm, r53: p.r53}
}

func ensureTestCertificateArn(sp *subproc, properties Properties) (string, error) {
	certificateArn, err := sp.createCertificateAndTags(properties)
	if err != nil {
		return "", errors.Wrapf(err, "could not create certificate for %+v", properties)
	}
	return certificateArn, nil
}

func (p *subproc) tearDownCreation(t *testing.T, certificateArn string, properties Properties) {
	err := p.deleteRecordSetGroup(certificateArn, properties)
	if err != nil {
		t.Errorf("could not delete the records: %v", err)
	}
}

func (p *subproc) deleteTestCertificate(t *testing.T, certificateArn string) {
	_, err := p.acm.DeleteCertificateRequest(&acm.DeleteCertificateInput{
		CertificateArn: &certificateArn,
	}).Send()
	if err != nil {
		t.Fatalf("could not delete the certificate %s due to err %s", certificateArn, err)
	}
}

func hasCnameRecordFor(changes []route53.Change, domain string) bool {
	for _, change := range changes {
		set := change.ResourceRecordSet
		if set.Type == route53.RRTypeCname && strings.HasSuffix(*set.Name, domain) {
			return true
		}
	}
	return false
}

func hasCaaRecordFor(changes []route53.Change, domain string) bool {
	for _, change := range changes {
		set := change.ResourceRecordSet
		if set.Type == route53.RRTypeCaa && *set.Name == domain {
			return true
		}
	}
	return false
}

func (p *subproc) deleteOneCaaRecord(certificateArn string, properties Properties) error {
	changes, err := p.generateChanges(certificateArn, properties, DeleteAction, caaSpec)
	if err != nil {
		return err
	}
	return p.deleteChanges(properties.HostedZoneId, changes[0:1])
}
