package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"

	esasg "github.com/mintel/elasticsearch-asg"
	"github.com/mintel/elasticsearch-asg/pkg/str"
)

func MakeCloudwatchData(nodes map[string]*esasg.Node) []*cloudwatch.MetricDatum {
	timestamp := time.Now()

	roles := []string{"all", "coordinate"}
	var clusterName string
	for _, node := range nodes {
		clusterName = node.ClusterName
		roles = append(roles, node.Roles...)
	}
	roles = str.Uniq(roles...)
	collectors := make(map[string]*statsCollector, len(roles))
	for _, role := range roles {
		collectors[role] = newStatsCollector()
	}

	for _, node := range nodes {
		if len(node.Roles) > 0 {
			for _, role := range node.Roles {
				collectors[role].Add(node)
			}
		} else {
			collectors["coordinate"].Add(node)
		}
		collectors["all"].Add(node)
	}

	metrics := make([]*cloudwatch.MetricDatum, 0)
	for role, collector := range collectors {
		dimensions := []*cloudwatch.Dimension{
			&cloudwatch.Dimension{
				Name:  aws.String("ClusterName"),
				Value: aws.String(clusterName),
			},
			&cloudwatch.Dimension{
				Name:  aws.String("Role"),
				Value: aws.String(role),
			},
		}
		metrics = append(metrics, collector.Metrics(dimensions, timestamp)...)
	}
	return metrics
}

// statsCollector aggregates metrics about Elasticsearch nodes useful
// for autoscaling.
type statsCollector struct {
	// count of nodes added
	count int

	// JVM heap size
	maxHeapBytes []float64

	// JVM heap used
	usedHeapBytes []float64

	jvmPools map[string]*struct {
		Used     []float64
		Max      []float64
		PeakUsed []float64
		PeakMax  []float64
	}

	// Stats about garbage collectors
	garbageCollectors map[string]*struct {
		Count []float64
		Time  []float64
	}

	// Elasticsearch disk size in bytes
	maxFSBytes []float64

	// Elasticsearch disk available bytes
	availableFSBytes []float64

	// Node is excluded from shard allocation
	excludedFromAllocation []bool
}

func newStatsCollector() *statsCollector {
	return &statsCollector{
		maxHeapBytes:  make([]float64, 0),
		usedHeapBytes: make([]float64, 0),
		jvmPools: make(map[string]*struct {
			Used     []float64
			Max      []float64
			PeakUsed []float64
			PeakMax  []float64
		}),
		garbageCollectors: make(map[string]*struct {
			Count []float64
			Time  []float64
		}),
		maxFSBytes:             make([]float64, 0),
		availableFSBytes:       make([]float64, 0),
		excludedFromAllocation: make([]bool, 0),
	}
}

// Add appends stats about an Elasticsearch node.
func (s *statsCollector) Add(node *esasg.Node) {
	s.count++

	s.maxHeapBytes = append(s.maxHeapBytes, float64(node.Stats.JVM.Mem.HeapMaxInBytes))
	s.usedHeapBytes = append(s.usedHeapBytes, float64(node.Stats.JVM.Mem.HeapUsedInBytes))

	for name, p := range node.Stats.JVM.Mem.Pools {
		d, ok := s.jvmPools[name]
		if !ok {
			d = &struct {
				Used     []float64
				Max      []float64
				PeakUsed []float64
				PeakMax  []float64
			}{
				Used:     make([]float64, 0),
				Max:      make([]float64, 0),
				PeakUsed: make([]float64, 0),
				PeakMax:  make([]float64, 0),
			}
			s.jvmPools[name] = d
		}
		d.Used = append(d.Used, float64(p.UsedInBytes))
		d.Max = append(d.Max, float64(p.MaxInBytes))
		d.PeakUsed = append(d.PeakUsed, float64(p.PeakUsedInBytes))
		d.PeakMax = append(d.PeakMax, float64(p.PeakMaxInBytes))
	}

	for name, c := range node.Stats.JVM.GC.Collectors {
		d, ok := s.garbageCollectors[name]
		if !ok {
			d = &struct {
				Count []float64
				Time  []float64
			}{
				Count: make([]float64, 0),
				Time:  make([]float64, 0),
			}
			s.garbageCollectors[name] = d
		}
		d.Count = append(d.Count, float64(c.CollectionCount))
		d.Time = append(d.Time, float64(c.CollectionTimeInMillis))
	}

	if str.In("data", node.Roles...) {
		s.maxFSBytes = append(s.maxFSBytes, float64(node.Stats.FS.Total.TotalInBytes))
		s.availableFSBytes = append(s.availableFSBytes, float64(node.Stats.FS.Total.AvailableInBytes))
		s.excludedFromAllocation = append(s.excludedFromAllocation, node.ExcludedShardAllocation)
	} else {
		s.maxFSBytes = append(s.maxFSBytes, 0)
		s.availableFSBytes = append(s.availableFSBytes, 0)
		s.excludedFromAllocation = append(s.excludedFromAllocation, false)
	}
}

// Metrics returns the CloudWatch metric data points.
func (s *statsCollector) Metrics(dimensions []*cloudwatch.Dimension, timestamp time.Time) []*cloudwatch.MetricDatum {
	metrics := make([]*cloudwatch.MetricDatum, 0)
	if s.count == 0 {
		return metrics
	}

	maxHeapBytes := sum(s.maxHeapBytes...)
	usedHeapBytes := sum(s.usedHeapBytes...)
	countExcludedFromAllocation := countTrue(s.excludedFromAllocation...)

	metrics = append(metrics,
		&cloudwatch.MetricDatum{
			Timestamp:         aws.Time(timestamp),
			MetricName:        aws.String("CountNodes"),
			Dimensions:        dimensions,
			Unit:              aws.String(cloudwatch.StandardUnitCount),
			StorageResolution: aws.Int64(1),
			Value:             aws.Float64(float64(s.count)),
		},
		&cloudwatch.MetricDatum{
			Timestamp:         aws.Time(timestamp),
			MetricName:        aws.String("CountExcludedFromAllocation"),
			Dimensions:        dimensions,
			Unit:              aws.String(cloudwatch.StandardUnitCount),
			StorageResolution: aws.Int64(1),
			Value:             aws.Float64(float64(countExcludedFromAllocation)),
		},
		&cloudwatch.MetricDatum{
			Timestamp:         aws.Time(timestamp),
			MetricName:        aws.String("JVMTotal"),
			Dimensions:        dimensions,
			Unit:              aws.String(cloudwatch.StandardUnitBytes),
			StorageResolution: aws.Int64(1),
			Value:             aws.Float64(maxHeapBytes),
		},
		&cloudwatch.MetricDatum{
			Timestamp:         aws.Time(timestamp),
			MetricName:        aws.String("JVMUsed"),
			Dimensions:        dimensions,
			Unit:              aws.String(cloudwatch.StandardUnitBytes),
			StorageResolution: aws.Int64(1),
			Value:             aws.Float64(usedHeapBytes),
		},
		&cloudwatch.MetricDatum{
			Timestamp:         aws.Time(timestamp),
			MetricName:        aws.String("JVMUtilization"),
			Dimensions:        dimensions,
			Unit:              aws.String(cloudwatch.StandardUnitPercent),
			StorageResolution: aws.Int64(1),
			Value:             aws.Float64(usedHeapBytes / maxHeapBytes * 100), // CloudWatch percents are int 0-100
		},
	)

	for name, p := range s.jvmPools {
		name = strings.Title(name)
		used := sum(p.Used...)
		max := sum(p.Max...)
		peakUsed := sum(p.PeakUsed...)
		peakMax := sum(p.PeakMax...)
		metrics = append(metrics,
			&cloudwatch.MetricDatum{
				Timestamp:         aws.Time(timestamp),
				MetricName:        aws.String(fmt.Sprintf("JVM%sPoolMax", name)),
				Dimensions:        dimensions,
				Unit:              aws.String(cloudwatch.StandardUnitBytes),
				StorageResolution: aws.Int64(1),
				Value:             aws.Float64(max),
			},
			&cloudwatch.MetricDatum{
				Timestamp:         aws.Time(timestamp),
				MetricName:        aws.String(fmt.Sprintf("JVM%sPoolUsed", name)),
				Dimensions:        dimensions,
				Unit:              aws.String(cloudwatch.StandardUnitBytes),
				StorageResolution: aws.Int64(1),
				Value:             aws.Float64(used),
			},
			&cloudwatch.MetricDatum{
				Timestamp:         aws.Time(timestamp),
				MetricName:        aws.String(fmt.Sprintf("JVM%sPoolPeakMax", name)),
				Dimensions:        dimensions,
				Unit:              aws.String(cloudwatch.StandardUnitBytes),
				StorageResolution: aws.Int64(1),
				Value:             aws.Float64(peakMax),
			},
			&cloudwatch.MetricDatum{
				Timestamp:         aws.Time(timestamp),
				MetricName:        aws.String(fmt.Sprintf("JVM%sPoolPeakUsed", name)),
				Dimensions:        dimensions,
				Unit:              aws.String(cloudwatch.StandardUnitBytes),
				StorageResolution: aws.Int64(1),
				Value:             aws.Float64(peakUsed),
			},
			&cloudwatch.MetricDatum{
				Timestamp:         aws.Time(timestamp),
				MetricName:        aws.String(fmt.Sprintf("JVM%sPoolUtilization", name)),
				Dimensions:        dimensions,
				Unit:              aws.String(cloudwatch.StandardUnitPercent),
				StorageResolution: aws.Int64(1),
				Value:             aws.Float64(used / maxHeapBytes * 100), // CloudWatch percents are int 0-100
			},
		)
	}

	for name, c := range s.garbageCollectors {
		name = strings.Title(name)
		count := sum(c.Count...)
		time := sum(c.Time...)
		metrics = append(metrics,
			&cloudwatch.MetricDatum{
				Timestamp:         aws.Time(timestamp),
				MetricName:        aws.String(fmt.Sprintf("GC%sCount", name)),
				Dimensions:        dimensions,
				Unit:              aws.String(cloudwatch.StandardUnitCount),
				StorageResolution: aws.Int64(1),
				Value:             aws.Float64(count),
			},
			&cloudwatch.MetricDatum{
				Timestamp:         aws.Time(timestamp),
				MetricName:        aws.String(fmt.Sprintf("GC%sTime", name)),
				Dimensions:        dimensions,
				Unit:              aws.String(cloudwatch.StandardUnitCountSecond),
				StorageResolution: aws.Int64(1),
				Value:             aws.Float64(time / 1000),
			},
		)
	}

	if sumMaxFSBytes := sum(s.maxFSBytes...); sumMaxFSBytes > 0 {
		sumAvailableFSBytes := sum(s.availableFSBytes...)
		metrics = append(metrics,
			&cloudwatch.MetricDatum{
				Timestamp:         aws.Time(timestamp),
				MetricName:        aws.String("FSMaxBytes"),
				Dimensions:        dimensions,
				Unit:              aws.String(cloudwatch.StandardUnitBytes),
				StorageResolution: aws.Int64(1),
				Value:             aws.Float64(sumMaxFSBytes),
			},
			&cloudwatch.MetricDatum{
				Timestamp:         aws.Time(timestamp),
				MetricName:        aws.String("FSAvailableBytes"),
				Dimensions:        dimensions,
				Unit:              aws.String(cloudwatch.StandardUnitBytes),
				StorageResolution: aws.Int64(1),
				Value:             aws.Float64(sumAvailableFSBytes),
			},
			&cloudwatch.MetricDatum{
				Timestamp:         aws.Time(timestamp),
				MetricName:        aws.String("FSUtilization"),
				Dimensions:        dimensions,
				Unit:              aws.String(cloudwatch.StandardUnitPercent),
				StorageResolution: aws.Int64(1),
				Value:             aws.Float64((1.0 - (sumAvailableFSBytes / sumMaxFSBytes)) * 100), // CloudWatch percents are int 0-100
			},
		)
	}

	return metrics
}

func sum(d ...float64) float64 {
	var sum float64
	for _, v := range d {
		sum += v
	}
	return sum
}

func countTrue(b ...bool) int {
	sum := 0
	for _, v := range b {
		if v {
			sum++
		}
	}
	return sum
}
