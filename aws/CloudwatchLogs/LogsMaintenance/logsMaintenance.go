package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/codesmith-gmbh/forge/aws/common"
	"github.com/pkg/errors"
	"os"
)

var (
	snsAlertTopicArn             = os.Getenv("SNS_ALERT_TOPIC_ARN")
	log                          = common.MustSugaredLogger()
	DefaultRetentionInDays int64 = 90
)

func main() {
	defer common.SyncSugaredLogger(log)
	cfg := common.MustConfig()
	p := &proc{sns: sns.New(cfg), ec2: ec2.New(cfg)}
	lambda.Start(p.processEvent)
}

func (p *proc) processEvent(event events.CloudWatchEvent) error {
	log.Infow("maintenance", "event", event)
	regions, err := p.ec2.DescribeRegionsRequest(&ec2.DescribeRegionsInput{}).Send()
	if err != nil {
		return errors.Wrapf(err, "could not fetch the list of regions")
	}
	problems := make(map[string][]problem)
	problemCount := 0
	for _, region := range regions.Regions {
		log.Infow("Region Maintenance", "region", region)
		regionName := *region.RegionName
		ps := p.checkLogGroupExpiration(regionName)
		problemCount += len(ps)
		problems[regionName] = ps
	}
	if problemCount > 0 {
		if err := p.alert(problems, problemCount); err != nil {
			return err
		}
	}
	return nil
}

type proc struct {
	sns *sns.SNS
	ec2 *ec2.EC2
}

type problem struct {
	LogGroupName, Error string
}

func (p *proc) checkLogGroupExpiration(region string) []problem {
	problems := make([]problem, 0, 10)
	cfg := common.MustConfig(external.WithRegion(region))
	logs := cloudwatchlogs.New(cfg)
	grps, err := logs.DescribeLogGroupsRequest(&cloudwatchlogs.DescribeLogGroupsInput{}).Send()
	if err != nil {
		problems = append(problems, problem{Error: err.Error()})
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
				}).Send()
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
		}).Send()
		if err != nil {
			problems = append(problems, problem{Error: err.Error()})
		}
	}
	return problems
}

func (p *proc) alert(problems map[string][]problem, problemCount int) error {
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
	}).Send()
	return err
}
