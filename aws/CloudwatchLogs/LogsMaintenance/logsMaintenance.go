package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/codesmith-gmbh/cgc/cgclog"
	"github.com/pkg/errors"
	"os"
)

var (
	snsAlertTopicArn             = os.Getenv("SNS_ALERT_TOPIC_ARN")
	log                          = cgclog.MustSugaredLogger()
	DefaultRetentionInDays int64 = 90
)

func main() {
	defer cgclog.SyncSugaredLogger(log)
	p := newProc()
	lambda.Start(p.ProcessEvent)
}

type EventProcessor interface {
	ProcessEvent(ctx context.Context, event events.CloudWatchEvent) error
}

type ConstantErrorEventProcessor struct {
	Error error
}

func (p *ConstantErrorEventProcessor) ProcessEvent(ctx context.Context, event events.CloudWatchEvent) error {
	return p.Error
}

type proc struct {
	sns *sns.Client
	ec2 *ec2.Client
}

func newProc() EventProcessor {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		return &ConstantErrorEventProcessor{Error: err}
	}
	return newProcFromConfig(cfg)
}

func newProcFromConfig(cfg aws.Config) *proc {
	return &proc{sns: sns.New(cfg), ec2: ec2.New(cfg)}
}

func (p *proc) ProcessEvent(ctx context.Context, event events.CloudWatchEvent) error {
	log.Infow("maintenance", "event", event)
	regions, err := p.ec2.DescribeRegionsRequest(&ec2.DescribeRegionsInput{}).Send(ctx)
	if err != nil {
		return errors.Wrapf(err, "could not fetch the list of regions")
	}
	problems := make(map[string][]problem)
	problemCount := 0
	for _, region := range regions.Regions {
		log.Infow("Region Maintenance", "region", region)
		regionName := *region.RegionName
		ps := p.checkLogGroupExpiration(ctx, regionName)
		problemCount += len(ps)
		problems[regionName] = ps
	}
	if problemCount > 0 {
		if err := p.alert(ctx, problems, problemCount); err != nil {
			return err
		}
	}
	return nil
}

type problem struct {
	LogGroupName, Error string
}

func (p *proc) checkLogGroupExpiration(ctx context.Context, region string) []problem {
	problems := make([]problem, 0, 10)
	cfg, err := external.LoadDefaultAWSConfig(external.WithRegion(region))
	if err != nil {
		return append(problems, problem{Error: err.Error()})
	}
	logs := cloudwatchlogs.New(cfg)
	grps, err := logs.DescribeLogGroupsRequest(&cloudwatchlogs.DescribeLogGroupsInput{}).Send(ctx)
	if err != nil {
		return append(problems, problem{Error: err.Error()})
	}
	for {
		for _, grp := range grps.LogGroups {
			if grp.RetentionInDays == nil || *grp.RetentionInDays != DefaultRetentionInDays {
				log.Debugw("LogGroup Retention Policy",
					"region", region,
					"logGroupName", *grp.LogGroupName,
					"retentionInDays", DefaultRetentionInDays)
				_, err := logs.PutRetentionPolicyRequest(&cloudwatchlogs.PutRetentionPolicyInput{
					LogGroupName:    grp.LogGroupName,
					RetentionInDays: &DefaultRetentionInDays,
				}).Send(ctx)
				if err != nil {
					problems = append(problems, problem{LogGroupName: *grp.LogGroupName, Error: err.Error()})
				}
			}
		}
		nextToken := grps.NextToken
		if nextToken == nil {
			break
		}
		grps, err = logs.DescribeLogGroupsRequest(&cloudwatchlogs.DescribeLogGroupsInput{
			NextToken: grps.NextToken,
		}).Send(ctx)
		if err != nil {
			return append(problems, problem{Error: err.Error()})
		}
	}
	return problems
}

func (p *proc) alert(ctx context.Context, problems map[string][]problem, problemCount int) error {
	log.Debugw("problems", "count", problemCount)
	subject := fmt.Sprintf("CloudwatchLogs expiration checker and gc has %d problems", problemCount)
	msg, err := json.Marshal(problems)
	msgText := string(msg)
	if err != nil {
		return errors.Wrapf(err, "could not marshal the %d problems", problemCount)
	}
	_, err = p.sns.PublishRequest(&sns.PublishInput{
		Subject:  &subject,
		Message:  &msgText,
		TopicArn: &snsAlertTopicArn,
	}).Send(ctx)
	return err
}
