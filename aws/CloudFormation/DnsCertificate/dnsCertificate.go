package main

import (
	"context"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	forgeacm "github.com/codesmith-gmbh/forge/aws/acm"
	"github.com/codesmith-gmbh/forge/aws/common"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

type proc struct {
	acmService func(properties forgeacm.Properties) (*acm.ACM, error)
}

// The lambda is started using the AWS lambda go sdk. The handler function
// does the actual work of creating the certificate. Cloudformation sends
// an event to signify that a resources must be created, updated or
// deleted.
func main() {
	p := &proc{acmService}
	lambda.Start(cfn.LambdaWrap(p.processEvent))
}

func decodeProperties(input map[string]interface{}) (forgeacm.Properties, error) {
	var properties forgeacm.Properties
	if err := mapstructure.Decode(input, &properties); err != nil {
		return properties, err
	}
	return properties, nil
}

// When processing an event, we first decode the resource properties and
// create a acm client client. We have then 3 cases:
//
// 1. Delete: The delete case it self has 2 sub cases: if the physical
//    resource id is a failure id, then this is a NOP, otherwise we delete
//    the certificate.
// 2. Create: In that case, we proceed to create the certificate,
//    add tags if applicable and collect the DNS CNAME records to construct
//    the attributes of the resource.
// 3. Update: If only the tags have changed, we update them; otherwise, the update
//    requires a replacement and the resource is normally created.
func (p *proc) processEvent(_ context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	properties, err := decodeProperties(event.ResourceProperties)
	if err != nil {
		return "", nil, err
	}
	acms, err := p.acmService(properties)
	if err != nil {
		return "", nil, err
	}
	switch event.RequestType {
	case cfn.RequestDelete:
		if common.IsCertificateArn(event.PhysicalResourceID) {
			_, err := acms.DeleteCertificateRequest(&acm.DeleteCertificateInput{
				CertificateArn: &event.PhysicalResourceID,
			}).Send()
			if err != nil {
				return event.PhysicalResourceID, nil, errors.Wrapf(err, "could not delete the certificate %s", event.PhysicalResourceID)
			}
		}
		return event.PhysicalResourceID, nil, nil
	case cfn.RequestCreate:
		return forgeacm.CreateCertificate(acms, properties)
	case cfn.RequestUpdate:
		oldProperties, err := decodeProperties(event.OldResourceProperties)
		if err != nil {
			return event.PhysicalResourceID, nil, err
		}
		if onlyTagsChanged(event, oldProperties, properties) {
			data, err := updateTags(acms, event, properties)
			return event.PhysicalResourceID, data, err
		} else {
			return forgeacm.CreateCertificate(acms, properties)
		}
	default:
		return common.UnknownRequestType(event)
	}
}

// ### Update
//
// As explained above, we update if and only if the tags are the only
// properties to have changed. For this purpose, we check the equality of
// all the other properties. `SubjectAlternativeNames` is considered a set.
//
// Note that we do not test the tags themselves: it is not necessary as
// cloudformation sends an update request only if at least one property has
// changed.
func onlyTagsChanged(event cfn.Event, oldProperties, properties forgeacm.Properties) bool {
	return properties.DomainName == oldProperties.DomainName &&
		common.IsSameRegion(event, oldProperties.Region, properties.Region) &&
		sameSubjectAlternativeNames(properties.SubjectAlternativeNames, oldProperties.SubjectAlternativeNames)
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
	var san1Set = make(map[string]struct{})
	for _, san := range san1 {
		san1Set[san] = struct{}{}
	}
	for _, san := range san2 {
		_, ok := san1Set[san]
		if !ok {
			return false
		}
	}
	return true
}

// Updating is quite straightforward: we delete all the tags before
// recreating them. We must gather the CNAME records to send as attribute
// to the response.
func updateTags(acms *acm.ACM, event cfn.Event, properties forgeacm.Properties) (map[string]interface{}, error) {
	// 1. we first fetch the tags.
	tags, err := acms.ListTagsForCertificateRequest(&acm.ListTagsForCertificateInput{
		CertificateArn: &event.PhysicalResourceID,
	}).Send()
	if err != nil {
		return nil, errors.Wrapf(err, "could not list tags for certificate %s", event.PhysicalResourceID)
	}
	// 2. we remove them all.
	_, err = acms.RemoveTagsFromCertificateRequest(&acm.RemoveTagsFromCertificateInput{
		CertificateArn: &event.PhysicalResourceID,
		Tags:           tags.Tags,
	}).Send()
	if err != nil {
		return nil, errors.Wrapf(err, "could not remove tags for certificate %s", event.PhysicalResourceID)
	}
	// 3. we create the new tags.
	_, err = acms.AddTagsToCertificateRequest(&acm.AddTagsToCertificateInput{
		CertificateArn: &event.PhysicalResourceID,
		Tags:           properties.Tags,
	}).Send()
	if err != nil {
		return nil, errors.Wrapf(err, "could not add tags for certificate %s", event.PhysicalResourceID)
	}
	// 4. we gather the data.
	data, err := forgeacm.DataForResource(acms, &event.PhysicalResourceID, properties)
	if err != nil {
		return nil, err
	}
	// 5. finally, we send back the response.
	return data, nil
}

// ### SDK client
//
// We use the
// [ACM sdk v2](https://github.com/aws/aws-sdk-go-v2/tree/master/service/acm)
// to create the certificate. The client is created with the default
// credential chain loader, if need be with the supplied region.
func acmService(properties forgeacm.Properties) (*acm.ACM, error) {
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
