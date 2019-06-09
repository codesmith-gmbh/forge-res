// # PipelineTrigger
//
// The PipelineTrigger lambda function is used as trigger for code commit repository to trigger a CodePipeline pipeline.
// The source of the pipeline must be a be found at the key `<pipeline-name>/trigger.zip` in an S3 bucket configured
// via the environment variable `EVENTS_BUCKET_NAME`.
//
// This lambda function is part of the CodePipeline tooling. Please refer to the CodePipeline tooling for more information.
package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/codesmith-gmbh/cgc/cgcaws"
	"github.com/codesmith-gmbh/cgc/cgclog"
	"github.com/pkg/errors"
	"os"
	"strings"
)

type Settings struct {
	Pipeline string `json:"pipeline"`
	OnTag    bool   `json:"onTag,omitempty"`
	OnCommit bool   `json:"onCommit,omitempty"`
}

const EventsBucketName = "EVENTS_BUCKET_NAME"
const Buildspec = `version: 0.2

phases:
  build:
    commands:
      - bash checkout.sh
artifacts:
  type: zip
  files:
    - "**/*"
  base-directory: repo`

const CheckoutSh = `#!/bin/bash

git config --global credential.helper '!aws codecommit credential-helper $@'
git config --global credential.UseHttpPath true

git clone --shallow-submodules https://git-codecommit.%s.amazonaws.com/v1/repos/%s repo
cd repo
%s
cd
`

var log = cgclog.MustSugaredLogger()

func main() {
	defer cgclog.SyncSugaredLogger(log)
	cfg := cgcaws.MustConfig()
	p := newProc(cfg)
	lambda.Start(p.processEvent)
}

type proc struct {
	s3 *s3.Client
}

func newProc(cfg aws.Config) *proc {
	return &proc{s3: s3.New(cfg)}
}

func (p *proc) processEvent(ctx context.Context, event events.CodeCommitEvent) (events.CodeCommitEvent, error) {
	log.Debugw("received CodeCommit event", "event", event)
	commit := event.Records[0]
	settings, err := settings(commit.CustomData)
	if err != nil {
		return event, err
	}

	awsRegion := commit.AWSRegion
	repository := extractRepository(commit)
	ref := commit.CodeCommit.References[0]
	log.Debugw("repository data",
		"repository", repository,
		"ref", ref,
		"awsRegion", awsRegion)

	switch {
	case isCommit(ref) && settings.OnCommit:
		if err := p.triggerPipeline(ctx, awsRegion, repository, settings.Pipeline, "git checkout "+ref.Commit); err != nil {
			return event, err
		}
	case isTag(ref) && settings.OnTag:
		if err := p.triggerPipeline(ctx, awsRegion, repository, settings.Pipeline, "git checkout "+tag(ref)); err != nil {
			return event, err
		}
	default:
		log.Info("no trigger")
	}

	return event, nil
}

func settings(input string) (Settings, error) {
	var settings Settings
	if err := json.Unmarshal([]byte(input), &settings); err != nil {
		return settings, errors.Wrapf(err, "could not unmarshall settings: %s", input)
	}
	log.Debugw("fetched settings", "settings", settings)
	return settings, nil
}

func extractRepository(commit events.CodeCommitRecord) string {
	idx := strings.LastIndex(commit.EventSourceARN, ":")
	return commit.EventSourceARN[idx+1:]
}

func isCommit(ref events.CodeCommitReference) bool {
	return strings.HasPrefix(ref.Ref, "refs/heads/")
}

func isTag(ref events.CodeCommitReference) bool {
	return strings.HasPrefix(ref.Ref, "refs/tags/")
}

func tag(ref events.CodeCommitReference) string {
	return ref.Ref[10:len(ref.Ref)]
}

func (p *proc) triggerPipeline(ctx context.Context, awsRegion, repository, pipeline, gitCheckoutCommand string) error {
	buf := new(bytes.Buffer)
	writer := zip.NewWriter(buf)
	var files = []struct {
		Name, Body string
	}{
		{"buildspec.yaml", Buildspec},
		{"checkout.sh", fmt.Sprintf(CheckoutSh, awsRegion, repository, gitCheckoutCommand)},
	}
	for _, file := range files {
		zipFile, err := writer.Create(file.Name)
		if err != nil {
			return errors.Wrapf(err, "unable to create the file %s in the zip archive", file.Name)
		}
		_, err = zipFile.Write([]byte(file.Body))
		if err != nil {
			return errors.Wrapf(err, "unable to write the content %s of the file %s in the zip archive", file.Body, file.Name)
		}
	}
	err := writer.Close()
	if err != nil {
		return errors.Wrap(err, "unable to close the zip archive")
	}
	bucket := os.Getenv(EventsBucketName)
	key := pipeline + "/trigger.zip"
	_, err = p.s3.PutObjectRequest(&s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   bytes.NewReader(buf.Bytes()),
	}).Send(ctx)
	if err != nil {
		return errors.Wrapf(err, "could not put the object %s on the bucket %s", key, bucket)
	}
	return nil
}
