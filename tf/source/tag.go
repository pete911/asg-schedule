package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"strconv"
	"strings"
)

// Tags represents ASG tags
type Tags map[string]string

// toTags converts AWS tags to Tags (easier to work with)
func toTags(tags []types.TagDescription) Tags {
	out := make(map[string]string)
	for _, tag := range tags {
		out[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	return out
}

// ToScaling searches for 'asg-schedule' tag and converts the value to Scaling
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

		key := strings.TrimSpace(kv[0])
		val, err := strToInt32(strings.TrimSpace(kv[1]))
		if err != nil {
			return Scaling{}, fmt.Errorf("invalid %s tag value %s - %s:%s: %w", tagKey, tagValue, key, kv[1], err)
		}

		switch key {
		case "desired":
			scaling.DesiredCapacity = val
		case "max":
			scaling.MaxSize = val
		case "min":
			scaling.MinSize = val
		default:
			return Scaling{}, fmt.Errorf("invalid %s tag value %s", tagKey, tagValue)
		}
	}
	return scaling, nil
}

// strToInt32 helper function to convert string value from tag to int32 - used by AWS
func strToInt32(s string) (int32, error) {
	i, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(i), nil
}
