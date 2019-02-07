package main

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	forgeacm "github.com/codesmith-gmbh/forge/aws/acm"
	"github.com/codesmith-gmbh/forge/aws/common"
	"github.com/pkg/errors"
	"strings"
	"testing"
)

func TestCertificateRegion(t *testing.T) {
	// 1. Correctness
	for i, test := range []struct {
		arn, region string
	}{
		{arn: "arn:aws:acm:eu-west-1:account:certificate/12345678-1234-1234-1234-123456789012", region: "eu-west-1"},
		{arn: "arn:aws:acm:us-east-1:account:certificate/12345678-1234-1234-1234-123456789012", region: "us-east-1"},
	} {
		region, err := certificateRegion(test.arn)
		if err != nil {
			t.Errorf("error in correctness test case %d, %v, %s", i, test, err)
		}
		if region != test.region {
			t.Errorf("correctness test case %d, expected: %s, got: %s, test case %v", i, test.region, region, test)
		}
	}

	// 2. Completeness
	for i, test := range []string{
		"StackId-ResourceId-12345678",
	} {
		_, err := certificateRegion(test)
		if err == nil {
			t.Errorf("completeness test case %d, %s", i, test)
		}
	}
}

type testProc struct {
	proc
	cfg     aws.Config
	certArn string
}

func (p *testProc) mustProperties(t *testing.T) Properties {
	properties := Properties{
		CertificateArn: p.certArn,
		HostedZoneName: "codesmith.ch.",
		WithCaaRecords: true,
	}
	err := p.fetchHostedZoneData(&properties)
	if err != nil {
		t.Fatal(err, "could not fetch hosted zone data")
	}
	return properties
}

func TestIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	cfg := common.MustConfig(external.WithSharedConfigProfile("codesmith"))
	cm := acm.New(cfg)
	certArn, err := ensureTestCertificateArn(cm)
	if err != nil {
		t.Fatal(err, "no certificate could be created")
	}
	acmService := func(properties Properties) (*acm.ACM, error) {
		return acm.New(cfg), nil
	}
	p := testProc{proc: proc{r53: route53.New(cfg), acmService: acmService}, cfg: cfg, certArn: certArn}
	t.Run("Creation Change Generation CAA", p.TestCreationChangeGenerationWithCaa)
	t.Run("Creation Change Generation No CAA", p.TestCreationChangeGenerationWithoutCaa)
	t.Run("Update Change Generation CAA", p.TestUpdateChangeGenerationCaa)
	t.Run("Creation/Deletion Standard", p.TestCreationDeletionStandard)
	t.Run("Creation/Deletion Failover", p.TestCreationDeletionFailover)
	t.Run("Faulty Resource Deletion", p.TestFaultyResourceDeletion)
}

func (p *testProc) TestCreationChangeGenerationWithCaa(t *testing.T) {
	properties := p.mustProperties(t)
	properties.WithCaaRecords = true
	changes, err := p.generateChanges(properties, route53.ChangeActionCreate, validationSpec(properties))
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
	properties := p.mustProperties(t)
	properties.WithCaaRecords = false
	changes, err := p.generateChanges(properties, route53.ChangeActionCreate, validationSpec(properties))
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
	properties := p.mustProperties(t)
	properties.WithCaaRecords = false
	changes, err := p.generateChanges(properties, route53.ChangeActionCreate, caaSpec)
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

func (p *testProc) TestCreationDeletionStandard(t *testing.T) {
	properties := p.mustProperties(t)
	changeId, _, err := p.createRecordSetGroup(properties)
	if err != nil {
		t.Fatalf("could not create the record set group: %v", err)
	}
	defer p.tearDownCreation(t, properties)
	if changeId == "" {
		t.Errorf("change could not be created")
	}
}

func (p *testProc) TestCreationDeletionFailover(t *testing.T) {
	properties := p.mustProperties(t)
	changeId, _, err := p.createRecordSetGroup(properties)
	if err != nil {
		t.Fatalf("could not create the record set group: %v", err)
	}
	defer p.tearDownCreation(t, properties)
	if changeId == "" {
		t.Errorf("change could not be created")
	}
	err = p.deleteOneCaaRecord(properties)
	if err != nil {
		t.Errorf("could not manually delete a CAA record")
	}
}

func (p *testProc) TestFaultyResourceDeletion(t *testing.T) {
	properties := p.mustProperties(t)
	err := p.deleteRecordSetGroup(properties)
	if err != nil {
		t.Errorf("could not deleting faulty resources")
	}
}

func (p *testProc) tearDownCreation(t *testing.T, properties Properties) {
	err := p.deleteRecordSetGroup(properties)
	if err != nil {
		t.Errorf("could not delete the records: %v", err)
	}
}

func (p *testProc) deleteOneCaaRecord(properties Properties) error {
	changes, err := p.generateChanges(properties, route53.ChangeActionDelete, caaSpec)
	if err != nil {
		return err
	}
	return p.deleteChanges(properties.HostedZoneId, changes[0:1])
}

var domainName = "test-forge.codesmith.ch"

func ensureTestCertificateArn(cm *acm.ACM) (string, error) {
	certs, err := cm.ListCertificatesRequest(&acm.ListCertificatesInput{
	}).Send()
	if err != nil {
		return "", errors.Wrap(err, "could not load certificates")
	}
	for _, cert := range certs.CertificateSummaryList {
		if *cert.DomainName == domainName {
			return *cert.CertificateArn, nil
		}
	}

	certificateArn, _, err := forgeacm.CreateCertificate(cm, forgeacm.Properties{
		DomainName:              domainName,
		SubjectAlternativeNames: []string{"test-forge-san.codesmith.ch"},
	})
	if err != nil {
		return "", errors.Wrapf(err, "could not create certificate for %s", domainName)
	}
	return certificateArn, nil
}
