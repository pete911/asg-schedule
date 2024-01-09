package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"strings"
	"time"
)

const tagKey = "asg-schedule"

type Client struct {
	asgClient   *autoscaling.Client
	asgPrefixes []string
}

// NewClient creates new autoscaling client that works against ASG that match supplied prefixes
func NewClient(asgPrefixes []string) (Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return Client{}, fmt.Errorf("load default aws config: %w", err)
	}
	return Client{asgClient: autoscaling.NewFromConfig(cfg), asgPrefixes: asgPrefixes}, nil
}

// ScaleDown sets ASGs desired, max and min capacity to 0 and creates 'asg-schedule' tag with previous settings
// only ASG that match ASG prefixes are scaled down
func (c Client) ScaleDown() error {
	asgs, err := c.listAutoscalingGroups()
	if err != nil {
		return err
	}

	var errs []string
	for _, asg := range asgs {
		if err := c.scaleDown(asg); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) != 0 {
		return errors.New(strings.Join(errs, ", "))
	}
	return nil
}

// scaleDown sets ASG desired, max and min capacity to 0 and crates 'asg-schedule' tag with previous settings
func (c Client) scaleDown(asg ASG) error {
	asgIn := &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(asg.Name),
		DesiredCapacity:      aws.Int32(0),
		MaxSize:              aws.Int32(0),
		MinSize:              aws.Int32(0),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// scale down
	logger.Info(fmt.Sprintf("scaling down %s ASG", asg.Name))
	if _, err := c.asgClient.UpdateAutoScalingGroup(ctx, asgIn); err != nil {
		return err
	}

	// create/update tag with previous ASG size
	logger.Info(fmt.Sprintf("updating %s ASG %s tag", asg.Name, tagKey))
	tagsIn := &autoscaling.CreateOrUpdateTagsInput{Tags: []types.Tag{asg.Scaling.ScalingTag(asg.Name)}}
	if _, err := c.asgClient.CreateOrUpdateTags(ctx, tagsIn); err != nil {
		return err
	}
	return nil
}

// ScaleUp sets ASGs to desired, max and min capacity to those that are specified in the 'asg-schedule' tag
// only ASG that match ASG prefixes are scaled up
func (c Client) ScaleUp() error {
	asgs, err := c.listAutoscalingGroups()
	if err != nil {
		return err
	}

	var errs []string
	for _, asg := range asgs {
		if err := c.scaleUp(asg); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) != 0 {
		return errors.New(strings.Join(errs, ", "))
	}
	return nil
}

// scaleUp sets ASG desired, max and min capacity to those that are specified in the 'asg-schedule' tag
func (c Client) scaleUp(asg ASG) error {
	scaling, err := asg.Tags.ToScaling()
	if err != nil {
		return err
	}

	// get previous scaling config from the tag
	in := &autoscaling.UpdateAutoScalingGroupInput{
		DesiredCapacity: aws.Int32(scaling.DesiredCapacity),
		MaxSize:         aws.Int32(scaling.MaxSize),
		MinSize:         aws.Int32(scaling.MinSize),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// scale up
	in.AutoScalingGroupName = aws.String(asg.Name)
	logger.Info(fmt.Sprintf("scaling up %s ASG", asg.Name))
	if _, err := c.asgClient.UpdateAutoScalingGroup(ctx, in); err != nil {
		return err
	}
	// no need to delete tag, it will be updated when scaling down
	return nil
}

// listAutoscalingGroups that match any of the ASG prefixes
func (c Client) listAutoscalingGroups() ([]ASG, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var asgs []ASG
	in := &autoscaling.DescribeAutoScalingGroupsInput{}
	for {
		out, err := c.asgClient.DescribeAutoScalingGroups(ctx, in)
		if err != nil {
			return nil, err
		}
		// add only ASGs that match prefix
		for _, asg := range out.AutoScalingGroups {
			if !c.matchesASGPrefix(aws.ToString(asg.AutoScalingGroupName)) {
				continue
			}
			asgs = append(asgs, toASG(asg))
		}
		if aws.ToString(out.NextToken) == "" {
			break
		}
		in.NextToken = out.NextToken
	}
	return asgs, nil
}

// matchesASGPrefix checks if ASG name matches any of the ASG prefixes
func (c Client) matchesASGPrefix(asgName string) bool {
	for _, v := range c.asgPrefixes {
		if strings.HasPrefix(asgName, strings.TrimSpace(v)) {
			logger.Info(fmt.Sprintf("ASG %s matched %s prefix", asgName, v))
			return true
		}
	}
	logger.Info(fmt.Sprintf("ASG %s did not match [%v] prefixes", asgName, strings.Join(c.asgPrefixes, ", ")))
	return false
}
