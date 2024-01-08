package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"strconv"
	"strings"
	"time"
)

const tagKey = "asg-schedule"

type Client struct {
	asgClient   *autoscaling.Client
	asgPrefixes []string
}

func NewClient(asgPrefixes []string) (Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return Client{}, fmt.Errorf("load default aws config: %w", err)
	}
	return Client{asgClient: autoscaling.NewFromConfig(cfg), asgPrefixes: asgPrefixes}, nil
}

func (c Client) ScaleDown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	asgs, err := c.listAutoscalingGroups()
	if err != nil {
		return err
	}

	in := &autoscaling.UpdateAutoScalingGroupInput{
		DesiredCapacity: aws.Int32(0),
		MaxSize:         aws.Int32(0),
		MinSize:         aws.Int32(0),
	}

	for _, asg := range asgs {
		// scale down
		in.AutoScalingGroupName = aws.String(asg.Name)
		logger.Info(fmt.Sprintf("scaling down %s ASG", asg.Name))
		if _, err := c.asgClient.UpdateAutoScalingGroup(ctx, in); err != nil {
			return err
		}

		// create/update tag with previous ASG size
		logger.Info(fmt.Sprintf("updating %s ASG %s tag", asg.Name, tagKey))
		in := &autoscaling.CreateOrUpdateTagsInput{Tags: []types.Tag{asg.Scaling.ScalingTag(asg.Name)}}
		if _, err := c.asgClient.CreateOrUpdateTags(ctx, in); err != nil {
			return err
		}
	}
	return nil
}

func (c Client) ScaleUp() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	asgs, err := c.listAutoscalingGroups()
	if err != nil {
		return err
	}

	for _, asg := range asgs {
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

		// scale up
		in.AutoScalingGroupName = aws.String(asg.Name)
		logger.Info(fmt.Sprintf("scaling up %s ASG", asg.Name))
		if _, err := c.asgClient.UpdateAutoScalingGroup(ctx, in); err != nil {
			return err
		}
		// no need to delete tag, it will be updated when scaling down
	}
	return nil
}

type Tags map[string]string

func toTags(tags []types.TagDescription) Tags {
	out := make(map[string]string)
	for _, tag := range tags {
		out[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	return out
}

type ASG struct {
	Name    string
	Arn     string
	Scaling Scaling
	Tags    Tags
}

type Scaling struct {
	DesiredCapacity int32
	MaxSize         int32
	MinSize         int32
}

func (t Tags) ToScaling() (Scaling, error) {
	tagValue, ok := t[tagKey]
	if !ok {
		return Scaling{}, fmt.Errorf("tag %s not found", tagKey)
	}

	var scaling Scaling
	for _, item := range strings.Split(tagValue, ",") {
		kv := strings.Split(item, ":")
		if len(kv) != 2 {
			return Scaling{}, fmt.Errorf("invalid %s tag value %s", tagKey, tagValue)
		}

		v, err := strToInt32(kv[1])
		if err != nil {
			return Scaling{}, fmt.Errorf("invalid %s tag value %s - %s:%s: %w", tagKey, tagValue, kv[0], kv[1], err)
		}

		switch kv[0] {
		case "desired":
			scaling.DesiredCapacity = v
		case "max":
			scaling.MaxSize = v
		case "min":
			scaling.MinSize = v
		default:
			return Scaling{}, fmt.Errorf("invalid %s tag value %s", tagKey, tagValue)
		}
	}
	return scaling, nil
}

func strToInt32(s string) (int32, error) {
	i, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(i), nil
}

func (s Scaling) ScalingTag(asgName string) types.Tag {
	return types.Tag{
		Key:               aws.String(tagKey),
		Value:             aws.String(fmt.Sprintf("desired:%d,max:%d,min:%d", s.DesiredCapacity, s.MaxSize, s.MinSize)),
		PropagateAtLaunch: aws.Bool(false),
		ResourceType:      aws.String("auto-scaling-group"),
		ResourceId:        aws.String(asgName),
	}
}

func (c Client) toASGs(in []types.AutoScalingGroup) []ASG {
	var out []ASG
	for _, asg := range in {
		asgName := aws.ToString(asg.AutoScalingGroupName)
		if !c.matchesASGPrefix(asgName) {
			continue
		}

		out = append(out, ASG{
			Name: asgName,
			Arn:  aws.ToString(asg.AutoScalingGroupARN),
			Tags: toTags(asg.Tags),
			Scaling: Scaling{
				DesiredCapacity: aws.ToInt32(asg.DesiredCapacity),
				MaxSize:         aws.ToInt32(asg.MaxSize),
				MinSize:         aws.ToInt32(asg.MinSize),
			},
		})
	}
	return out
}

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
		asgs = append(asgs, c.toASGs(out.AutoScalingGroups)...)
		if aws.ToString(out.NextToken) == "" {
			break
		}
		in.NextToken = out.NextToken
	}
	return asgs, nil
}
