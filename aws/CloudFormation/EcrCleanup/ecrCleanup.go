package main

import (
	"context"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	awsecr "github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/codesmith-gmbh/cgc/cgccf"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

// The lambda is started using the AWS lambda go sdk. The handler function
// does the actual work of creating the log group. Cloudformation sends an
// event to signify that a resources must be created, updated or deleted.
func main() {
	p := newProc()
	cgccf.StartEventProcessor(p)
}

type proc struct {
	ecr *awsecr.Client
	cf  *cloudformation.Client
}

func newProc() cgccf.EventProcessor {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		return &cgccf.ConstantErrorEventProcessor{Error: err}
	}
	return newProcFromConfig(cfg)
}

func newProcFromConfig(cfg aws.Config) *proc {
	return &proc{ecr: awsecr.New(cfg), cf: cloudformation.New(cfg)}
}

type Properties struct {
	Repository string
}

func ecrCleanupProperties(input map[string]interface{}) (Properties, error) {
	var properties Properties
	if err := mapstructure.Decode(input, &properties); err != nil {
		return properties, err
	}
	return properties, nil
}

// To process an event, we first decode the resource properties and analyse
// the event. We have 2 cases.
//
// 1. Delete: The delete case it self has 3 sub cases:
//    1. the physical resource id is a failure id, then this is a NOP;
//    2. the stack is being deleted: in that case, we delete all the images in the
//       repository.
//    3. the stack is not being delete: it is a NOP as well.
// 2. Create, Update: In that case, it is a NOP, the physical ID is simply
//    the logical ID of the resource.
func (p *proc) ProcessEvent(ctx context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	properties, err := ecrCleanupProperties(event.ResourceProperties)
	if err != nil {
		return "", nil, err
	}
	switch event.RequestType {
	case cfn.RequestDelete:
		if hasValidPhysicalResourceID(event, properties) {
			stacks, err := p.cf.DescribeStacksRequest(&cloudformation.DescribeStacksInput{
				StackName: &event.StackID,
			}).Send(ctx)
			if err != nil {
				return event.PhysicalResourceID, nil, errors.Wrapf(err, "could not fetch the stack for the resource %s", event.PhysicalResourceID)
			}
			stackStatus := stacks.Stacks[0].StackStatus
			if stackStatus == cloudformation.StackStatusDeleteInProgress {
				if err = p.deleteImages(ctx, properties.Repository); err != nil {
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
		return event.LogicalResourceID, nil, errors.Errorf("unknown request type %s", event.RequestType)
	}
}

// We delete all the images in batches.
func (p *proc) deleteImages(ctx context.Context, repositoryName string) error {
	images, err := p.ecr.ListImagesRequest(&awsecr.ListImagesInput{
		RepositoryName: &repositoryName,
	}).Send(ctx)
	if err != nil {
		return errors.Wrapf(err, "could not fetch images for the repository %s", repositoryName)
	}
	for {
		if len(images.ImageIds) > 0 {
			_, err := p.ecr.BatchDeleteImageRequest(&awsecr.BatchDeleteImageInput{
				ImageIds:       images.ImageIds,
				RepositoryName: &repositoryName,
			}).Send(ctx)
			if err != nil {
				return errors.Wrapf(err, "could not delete images from the repository %s", repositoryName)
			}
		}
		if images.NextToken == nil {
			return nil
		}
		images, err = p.ecr.ListImagesRequest(&awsecr.ListImagesInput{
			RepositoryName: &repositoryName,
			NextToken:      images.NextToken,
		}).Send(ctx)
		if err != nil {
			return errors.Wrapf(err, "could not fetch images for the repository %s", repositoryName)
		}
	}
}

func physicalResourceID(event cfn.Event, properties Properties) string {
	return event.LogicalResourceID + ":" + properties.Repository
}

func hasValidPhysicalResourceID(event cfn.Event, properties Properties) bool {
	return event.PhysicalResourceID == physicalResourceID(event, properties)
}
