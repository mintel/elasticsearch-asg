package main

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	cache "github.com/patrickmn/go-cache"
)

// Cache EC2 instance ID => int count of vcpu cores.
var vcpuCache = cache.New(5*time.Minute, 10*time.Minute)

// GetInstanceVCPUCount gets the count of vCPUs for each EC2 instance in a list of instance IDs.
func GetInstanceVCPUCount(ec2Svc ec2iface.EC2API, IDs []string) (map[string]int, error) {
	out := make(map[string]int, len(IDs))
	toDesc := make([]*string, 0, len(IDs))
	for _, id := range IDs {
		if countPtr, ok := vcpuCache.Get(id); ok {
			out[id] = countPtr.(int)
		} else {
			toDesc = append(toDesc, aws.String(id))
		}
	}
	resp, err := ec2Svc.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: toDesc,
	})
	if err != nil {
		return nil, err
	}
	for _, r := range resp.Reservations {
		for _, i := range r.Instances {
			instanceID := *i.InstanceId
			vcpuCount := int(*i.CpuOptions.CoreCount * *i.CpuOptions.ThreadsPerCore)
			out[instanceID] = vcpuCount
			vcpuCache.SetDefault(instanceID, vcpuCount)
		}
	}
	return out, nil
}
