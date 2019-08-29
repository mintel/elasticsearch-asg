package main

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	cache "github.com/patrickmn/go-cache" // In-memory cache
	"go.uber.org/zap"                     // Logging

	// AWS clients and stuff
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"

	"github.com/mintel/elasticsearch-asg/pkg/metrics" // Prometheus metrics
)

// Cache EC2 instance ID => int count of vcpu cores.
var vcpuCache = cache.New(5*time.Minute, 10*time.Minute)

var (
	// DescEC2InstancesCacheSize is a Prometheus metric that tracks the number
	// of cached values for count of EC2 instance vCPUs.
	//nolint:deadcode,unused,varcheck
	vCPUCacheSize = promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: metrics.Namespace,
		Subsystem: subsystem,
		Name:      "instance_vcpus_cache_size",
		Help:      "Size of the cache of EC2 instance vCPU count information.",
	}, func() float64 {
		return float64(vcpuCache.ItemCount())
	})
)

// GetInstanceVCPUCount gets the count of vCPUs for each EC2 instance in a list of instance IDs.
func GetInstanceVCPUCount(ec2Svc ec2iface.EC2API, IDs []string) (map[string]int, error) {
	logger := zap.L()
	out := make(map[string]int, len(IDs))
	toDesc := make([]string, 0, len(IDs))
	for _, id := range IDs {
		if v, ok := vcpuCache.Get(id); ok {
			count := v.(int)
			logger.Debug("GetInstanceVCPUCount cache hit", zap.String("instance_id", id), zap.Int("count", count))
			out[id] = count
		} else {
			logger.Debug("GetInstanceVCPUCount cache miss", zap.String("instance_id", id))
			toDesc = append(toDesc, id)
		}
	}
	if len(toDesc) > 0 {
		logger.Debug("DescribeInstances", zap.Strings("instance_ids", toDesc))
		resp, err := ec2Svc.DescribeInstances(&ec2.DescribeInstancesInput{
			InstanceIds: aws.StringSlice(toDesc),
		})
		if err != nil {
			return nil, err
		}
		for _, r := range resp.Reservations {
			for _, i := range r.Instances {
				instanceID := *i.InstanceId
				count := int(*i.CpuOptions.CoreCount * *i.CpuOptions.ThreadsPerCore)
				out[instanceID] = count
				vcpuCache.SetDefault(instanceID, count)
				logger.Debug("got instance vcpu count", zap.String("instance_id", instanceID), zap.Int("count", count))
			}
		}
	}
	return out, nil
}
