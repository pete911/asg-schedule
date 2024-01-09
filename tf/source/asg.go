package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
)

// ASG represents Autoscaling group
type ASG struct {
	Name    string
	Arn     string
	Scaling Scaling
	Tags    Tags
}

// toASG converts AWS asg to ASG (easier to work with)
func toASG(asg types.AutoScalingGroup) ASG {
	return ASG{
		Name: aws.ToString(asg.AutoScalingGroupName),
		Arn:  aws.ToString(asg.AutoScalingGroupARN),
		Tags: toTags(asg.Tags),
		Scaling: Scaling{
			DesiredCapacity: aws.ToInt32(asg.DesiredCapacity),
			MaxSize:         aws.ToInt32(asg.MaxSize),
			MinSize:         aws.ToInt32(asg.MinSize),
		},
	}
}

// Scaling is scaling configuration of ASG
type Scaling struct {
	DesiredCapacity int32
	MaxSize         int32
	MinSize         int32
}

// ScalingTag returns current scaling configuration as AWS ASG tag
func (s Scaling) ScalingTag(asgName string) types.Tag {
	return types.Tag{
		Key:               aws.String(tagKey),
		Value:             aws.String(fmt.Sprintf("desired:%d,max:%d,min:%d", s.DesiredCapacity, s.MaxSize, s.MinSize)),
		PropagateAtLaunch: aws.Bool(false),
		ResourceType:      aws.String("auto-scaling-group"),
		ResourceId:        aws.String(asgName),
	}
}
