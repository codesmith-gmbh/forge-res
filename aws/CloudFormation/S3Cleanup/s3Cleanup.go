package main

import (
	"context"
	"github.com/codesmith-gmbh/cgc/cgcaws"
	"github.com/codesmith-gmbh/forge/aws/common"

	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

type proc struct {
	s3 *awss3.Client
	cf *cloudformation.Client
}

func main() {
	cfg := cgcaws.MustConfig()
	p := &proc{s3: awss3.New(cfg), cf: cloudformation.New(cfg)}
	lambda.Start(cfn.LambdaWrap(p.processEvent))
}

type Properties struct {
	ActiveOnlyOnStackDeletion string
	Bucket, Prefix            string
}

func s3CleanupProperties(input map[string]interface{}) (Properties, error) {
	var properties Properties
	if err := mapstructure.Decode(input, &properties); err != nil {
		return properties, err
	}
	if properties.Bucket == "" {
		return properties, errors.New("bucket name must be defined")
	}
	return properties, nil
}

// To process an event, we first decode the resource properties and analyse
// the event. We have 2 cases.
//
// 1. Delete: The delete case it self has 3 sub cases:
//    a. the physical resource id is not a valid physical ID for this resource, then this is a NOP;
//    b. the stack is being deleted: in that case, we delete all the objects with the given
//       path prefix from the S3 bucket or, if the path prefix is not defined, we delete
//       all the resources.
//    c. the stack is not being delete: it is a NOP as well.
// 2. Create, Update: In that case, it is a NOP, the physical ID is simply
//    the logical ID.
func (p *proc) processEvent(ctx context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	properties, err := s3CleanupProperties(event.ResourceProperties)
	if err != nil {
		return "", nil, err
	}
	switch event.RequestType {
	case cfn.RequestDelete:
		if hasValidPhysicalResourceID(event, properties) {
			shouldDelete, err := p.shouldDelete(ctx, event, properties)
			if err != nil {
				return event.PhysicalResourceID, nil, errors.Wrapf(err, "could not fetch the stack for the resource %s", event.PhysicalResourceID)
			}
			if shouldDelete {
				if err = p.deleteObjects(ctx, properties); err != nil {
					return event.PhysicalResourceID, nil, errors.Wrapf(err, "could not delete the images of the repository %s", event.PhysicalResourceID)
				}
			}
		}
		return event.PhysicalResourceID, nil, nil
	case cfn.RequestCreate:
		return physicalResourceID(event, properties), nil, nil
	case cfn.RequestUpdate:
		return physicalResourceID(event, properties), nil, nil
	default:
		return common.UnknownRequestType(event)
	}
}

func (p *proc) shouldDelete(ctx context.Context, event cfn.Event, properties Properties) (bool, error) {
	if properties.ActiveOnlyOnStackDeletion == "false" {
		return true, nil
	}
	stacks, err := p.cf.DescribeStacksRequest(&cloudformation.DescribeStacksInput{
		StackName: &event.StackID,
	}).Send(ctx)
	if err != nil {
		return false, errors.Wrapf(err, "could not fetch the stack for the resource %s", event.PhysicalResourceID)
	}
	stackStatus := stacks.Stacks[0].StackStatus
	return stackStatus == cloudformation.StackStatusDeleteInProgress, nil
}

func (p *proc) deleteObjects(ctx context.Context, properties Properties) error {
	versions, err := p.s3.ListObjectVersionsRequest(&awss3.ListObjectVersionsInput{
		Bucket: &properties.Bucket,
		Prefix: &properties.Prefix,
	}).Send(ctx)
	if err != nil {
		return errors.Wrapf(err, "could not fetch versions for the bucket %s", properties.Bucket)
	}
	quiet := true

	for {
		versionsLength := len(versions.Versions)
		if versionsLength > 0 {
			objects := make([]awss3.ObjectIdentifier, versionsLength)
			for i, version := range versions.Versions {
				objects[i] = awss3.ObjectIdentifier{
					Key:       version.Key,
					VersionId: version.VersionId,
				}
			}
			_, err = p.s3.DeleteObjectsRequest(&awss3.DeleteObjectsInput{
				Bucket: &properties.Bucket,
				Delete: &awss3.Delete{
					Objects: objects,
					Quiet:   &quiet,
				},
			}).Send(ctx)
			if err != nil {
				return errors.Wrapf(err, "could not delete objects from the s3 bucket %s", properties.Bucket)
			}
		}
		if *versions.IsTruncated {
			versions, err = p.s3.ListObjectVersionsRequest(&awss3.ListObjectVersionsInput{
				Bucket:          &properties.Bucket,
				Prefix:          &properties.Prefix,
				KeyMarker:       versions.NextKeyMarker,
				VersionIdMarker: versions.NextVersionIdMarker,
			}).Send(ctx)
			if err != nil {
				return errors.Wrapf(err, "could not fetch versions for the bucket %s", properties.Bucket)
			}
		} else {
			return nil
		}
	}
}

func physicalResourceID(event cfn.Event, properties Properties) string {
	return event.LogicalResourceID + ":" + properties.Bucket + ":" + properties.Prefix
}

func hasValidPhysicalResourceID(event cfn.Event, properties Properties) bool {
	return event.PhysicalResourceID == physicalResourceID(event, properties)
}
