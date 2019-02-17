package acm

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/pkg/errors"
	"time"
)

// The main data structure for the certificate resource is defined as a go
// struct. The struct mirrors the properties as defined above. We use the
// library [mapstructure](https://github.com/mitchellh/mapstructure) to
// decode the generic map from the cloudformation event to the struct.
type Properties struct {
	DomainName              string
	Region                  string
	SubjectAlternativeNames []string
	Tags                    []acm.Tag
}

// ### Creation
//
// We create the certificate with the certificate transparency logging
// enabled. If applicable, we add the tags to the certificate. Finally, we
// gather the CNAME record to be exported at attributes of the resource.
func CreateCertificate(acms *acm.ACM, properties Properties) (string, map[string]interface{}, error) {
	// 1. Create the certificate with certificate transparency logging enabled
	res, err := acms.RequestCertificateRequest(&acm.RequestCertificateInput{
		DomainName:       &properties.DomainName,
		ValidationMethod: acm.ValidationMethodDns,
		Options: &acm.CertificateOptions{
			CertificateTransparencyLoggingPreference: acm.CertificateTransparencyLoggingPreferenceEnabled,
		},
		SubjectAlternativeNames: properties.SubjectAlternativeNames,
	}).Send()
	if err != nil {
		return "", nil, errors.Wrap(err, "could not create the certificate")
	}

	// 2. If applicable, create the tags
	if len(properties.Tags) > 0 {
		_, err = acms.AddTagsToCertificateRequest(&acm.AddTagsToCertificateInput{
			CertificateArn: res.CertificateArn,
			Tags:           properties.Tags,
		}).Send()
		if err != nil {
			return *res.CertificateArn, nil, errors.Wrapf(err, "could not add tags to certificate %s", *res.CertificateArn)
		}
	}

	// 3. Fetch the certificate to get the domain validation information.
	data, err := DataForResource(acms, res.CertificateArn, properties)
	if err != nil {
		return *res.CertificateArn, nil, err
	}

	// 4. Construct the response to cloudformation.
	return *res.CertificateArn, data, nil
}

// Fetching for the data for the CNAME records requires a loop and waiting
// since those are created by AWS asynchronously and added to the
// certificate information only when they have been properly created. We
// wait at most 3 minutes with 3 seconds interval.
func DataForResource(acms *acm.ACM, certificateArn *string, properties Properties) (map[string]interface{}, error) {
OUTER:
	for i := 0; i < 60; i++ {
		cert, err := acms.DescribeCertificateRequest(&acm.DescribeCertificateInput{
			CertificateArn: certificateArn,
		}).Send()
		if err != nil {
			return nil, errors.Wrapf(err, "could not fetch certificate %s", *certificateArn)
		}
		fmt.Printf("Attempt %d: %+v\n", i, cert)
		options := cert.Certificate.DomainValidationOptions
		if options != nil && len(options) == len(properties.SubjectAlternativeNames)+1 {
			data := make(map[string]interface{}, 3*len(options))
			for _, option := range options {
				if option.ResourceRecord == nil {
					time.Sleep(3 * time.Second)
					continue OUTER
				}
				domainName := *option.DomainName
				data[domainName+"-RecordName"] = *option.ResourceRecord.Name
				data[domainName+"-RecordValue"] = *option.ResourceRecord.Value
				data[domainName+"-RecordType"] = option.ResourceRecord.Type
			}
			return data, nil
		}
		time.Sleep(3 * time.Second)
	}
	return nil, errors.Errorf("no DNS entries for certificate %s", *certificateArn)
}