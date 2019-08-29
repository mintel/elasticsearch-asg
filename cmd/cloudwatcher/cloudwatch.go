package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	// AWS clients and stuff
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"

	esasg "github.com/mintel/elasticsearch-asg"   // Complex Elasticsearch services
	"github.com/mintel/elasticsearch-asg/metrics" // Prometheus metrics
	"github.com/mintel/elasticsearch-asg/pkg/str" // String utilities
)

var (
	// PushMetricsTotal tracks the number of metric data points pushed to CloudWatch.
	PushMetricsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: subsystem,
		Name:      "pushed_metrics_total",
		Help:      "Count of metrics pushed to AWS CloudWatch.",
	})
)

// MakeCloudwatchData returns a list of CloudWatch
// metric data points related to an Elasticsearch cluster.
//
// The metrics are both in total, and broken out by Elasticsearch node role.
//
// Metrics include:
//
// - File system utilization (data nodes only)
// - JVM heap utilization (both in total, and per-memory pool)
// - JVM garbage collection stats
// - Linux Load
// - Count of nodes excluded from shard allocation
//
// Args:
// - nodes is the sort of output returned by esasg.ElasticsearchQueryService.Nodes().
// - vcpuCounts is the sort of output returned by GetInstanceVCPUCount().
func MakeCloudwatchData(nodes map[string]*esasg.Node, vcpuCounts map[string]int) []*cloudwatch.MetricDatum {
	timestamp := time.Now() // All metrics have a timestamp.

	// Get a set of all roles.
	roles := []string{
		"all",        // Fake role that matches all nodes.
		"coordinate", // Fake role that matches nodes that have no other roles.
	}
	var clusterName string // Also need the cluster name for later.
	for _, node := range nodes {
		if clusterName == "" {
			clusterName = node.ClusterName
		} else if clusterName != node.ClusterName {
			panic("got nodes from two different Elasticsearch clusters")
		}
		roles = append(roles, node.Roles...)
	}
	roles = str.Uniq(roles...)

	// Create a metrics collector for each role.
	collectors := make(map[string]*metricsCollector, len(roles))
	for _, role := range roles {
		collectors[role] = newMetricsCollector()
	}

	// Collect the metrics for each node.
	for instanceID, node := range nodes {
		vcpuCount, ok := vcpuCounts[instanceID]
		if !ok {
			panic("got nodes and vcpuCounts with different entries")
		}
		if len(node.Roles) > 0 {
			for _, role := range node.Roles {
				collectors[role].Add(node, vcpuCount)
			}
		} else {
			collectors["coordinate"].Add(node, vcpuCount)
		}
		collectors["all"].Add(node, vcpuCount)
	}

	// Assemble the metrics from all the collectors.
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

// metricsCollector aggregates CloudWatch metrics about a set of Elasticsearch nodes.
type metricsCollector struct {
	// count of nodes added
	count int

	// JVM heap size (all pools)
	maxJVMHeapBytes []float64

	// JVM heap used (all pools)
	usedJVMHeapBytes []float64

	// JVM heap memory pool sizes
	jvmHeapPools map[string]*struct {
		Used     []float64
		Max      []float64
		PeakUsed []float64
		PeakMax  []float64
	}

	// CPU load
	vcpuCounts []float64
	load1m     []float64
	load5m     []float64
	load15m    []float64

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

// newMetricsCollector returns a new metricsCollector.
func newMetricsCollector() *metricsCollector {
	return &metricsCollector{
		maxJVMHeapBytes:  make([]float64, 0),
		usedJVMHeapBytes: make([]float64, 0),
		vcpuCounts:       make([]float64, 0),
		load1m:           make([]float64, 0),
		load5m:           make([]float64, 0),
		load15m:          make([]float64, 0),
		jvmHeapPools: make(map[string]*struct {
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

// Add appends metrics about an Elasticsearch node to this collector.
func (s *metricsCollector) Add(node *esasg.Node, vcpuCount int) {
	s.count++ // Increment count of nodes collected.

	s.vcpuCounts = append(s.vcpuCounts, float64(vcpuCount)) // Add vCPU count.

	// Add 1m Load.
	load, ok := node.Stats.OS.CPU.LoadAverage["1m"]
	if !ok {
		panic("missing 1m load average")
	}
	s.load1m = append(s.load1m, load)

	// Add 5m Load.
	load, ok = node.Stats.OS.CPU.LoadAverage["5m"]
	if !ok {
		panic("missing 5m load average")
	}
	s.load5m = append(s.load5m, load)

	// Add 15m Load.
	load, ok = node.Stats.OS.CPU.LoadAverage["15m"]
	if !ok {
		panic("missing 15m load average")
	}
	s.load15m = append(s.load15m, load)

	// Add JVM heap totals
	s.maxJVMHeapBytes = append(s.maxJVMHeapBytes, float64(node.Stats.JVM.Mem.HeapMaxInBytes))
	s.usedJVMHeapBytes = append(s.usedJVMHeapBytes, float64(node.Stats.JVM.Mem.HeapUsedInBytes))

	// Add JVM heap stats for each memory pool
	for name, p := range node.Stats.JVM.Mem.Pools {
		d, ok := s.jvmHeapPools[name]
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
			s.jvmHeapPools[name] = d
		}
		d.Used = append(d.Used, float64(p.UsedInBytes))
		d.Max = append(d.Max, float64(p.MaxInBytes))
		d.PeakUsed = append(d.PeakUsed, float64(p.PeakUsedInBytes))
		d.PeakMax = append(d.PeakMax, float64(p.PeakMaxInBytes))
	}

	// Add JVM garbage collection stats.
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

	// Add filesystem stats for data nodes.
	if str.In("data", node.Roles...) {
		s.maxFSBytes = append(s.maxFSBytes, float64(node.Stats.FS.Total.TotalInBytes))
		s.availableFSBytes = append(s.availableFSBytes, float64(node.Stats.FS.Total.AvailableInBytes))
		s.excludedFromAllocation = append(s.excludedFromAllocation, node.ExcludedShardAllocation)
	} else {
		// For non-data nodes, append zeros just to keep slice sizes consistent.
		s.maxFSBytes = append(s.maxFSBytes, 0)
		s.availableFSBytes = append(s.availableFSBytes, 0)
		s.excludedFromAllocation = append(s.excludedFromAllocation, false)
	}
}

// Metrics returns the CloudWatch metric data points.
func (s *metricsCollector) Metrics(dimensions []*cloudwatch.Dimension, timestamp time.Time) []*cloudwatch.MetricDatum {
	metrics := make([]*cloudwatch.MetricDatum, 0) // The output slice of CloudWatch data points.

	if s.count == 0 { // Shortcut for no nodes Add()-ed.
		return metrics
	}

	// Sum vCPU counts.
	vcpuCount := sum(s.vcpuCounts...)

	// Sum loads.
	load1m := sum(s.load1m...)
	load5m := sum(s.load5m...)
	load15m := sum(s.load15m...)

	// Sum JVM heap totals.
	maxJVMHeapBytes := sum(s.maxJVMHeapBytes...)
	usedJVMHeapBytes := sum(s.usedJVMHeapBytes...)

	// Count nodes excluded from shard allocation.
	countExcludedFromAllocation := countTrue(s.excludedFromAllocation...)

	// Create some CloudWatch data points.
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
			MetricName:        aws.String("CountvCPU"),
			Dimensions:        dimensions,
			Unit:              aws.String(cloudwatch.StandardUnitCount),
			StorageResolution: aws.Int64(1),
			Value:             aws.Float64(vcpuCount),
		},
		&cloudwatch.MetricDatum{
			Timestamp:         aws.Time(timestamp),
			MetricName:        aws.String("Load1m"),
			Dimensions:        dimensions,
			Unit:              aws.String(cloudwatch.StandardUnitNone),
			StorageResolution: aws.Int64(1),
			Value:             aws.Float64(load1m),
		},
		&cloudwatch.MetricDatum{
			Timestamp:         aws.Time(timestamp),
			MetricName:        aws.String("Load5m"),
			Dimensions:        dimensions,
			Unit:              aws.String(cloudwatch.StandardUnitNone),
			StorageResolution: aws.Int64(1),
			Value:             aws.Float64(load5m),
		},
		&cloudwatch.MetricDatum{
			Timestamp:         aws.Time(timestamp),
			MetricName:        aws.String("Load15m"),
			Dimensions:        dimensions,
			Unit:              aws.String(cloudwatch.StandardUnitNone),
			StorageResolution: aws.Int64(1),
			Value:             aws.Float64(load15m),
		},
		&cloudwatch.MetricDatum{
			Timestamp:         aws.Time(timestamp),
			MetricName:        aws.String("Load1mUtilization"),
			Dimensions:        dimensions,
			Unit:              aws.String(cloudwatch.StandardUnitPercent),
			StorageResolution: aws.Int64(1),
			Value:             aws.Float64(load1m / vcpuCount * 100), // CloudWatch percents are int 0-100
		},
		&cloudwatch.MetricDatum{
			Timestamp:         aws.Time(timestamp),
			MetricName:        aws.String("Load5mUtilization"),
			Dimensions:        dimensions,
			Unit:              aws.String(cloudwatch.StandardUnitPercent),
			StorageResolution: aws.Int64(1),
			Value:             aws.Float64(load5m / vcpuCount * 100), // CloudWatch percents are int 0-100
		},
		&cloudwatch.MetricDatum{
			Timestamp:         aws.Time(timestamp),
			MetricName:        aws.String("Load15mUtilization"),
			Dimensions:        dimensions,
			Unit:              aws.String(cloudwatch.StandardUnitPercent),
			StorageResolution: aws.Int64(1),
			Value:             aws.Float64(load15m / vcpuCount * 100), // CloudWatch percents are int 0-100
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
			Value:             aws.Float64(maxJVMHeapBytes),
		},
		&cloudwatch.MetricDatum{
			Timestamp:         aws.Time(timestamp),
			MetricName:        aws.String("JVMUsed"),
			Dimensions:        dimensions,
			Unit:              aws.String(cloudwatch.StandardUnitBytes),
			StorageResolution: aws.Int64(1),
			Value:             aws.Float64(usedJVMHeapBytes),
		},
		&cloudwatch.MetricDatum{
			Timestamp:         aws.Time(timestamp),
			MetricName:        aws.String("JVMUtilization"),
			Dimensions:        dimensions,
			Unit:              aws.String(cloudwatch.StandardUnitPercent),
			StorageResolution: aws.Int64(1),
			Value:             aws.Float64(usedJVMHeapBytes / maxJVMHeapBytes * 100), // CloudWatch percents are int 0-100
		},
	)

	// Sum JVM heap memory pool stats.
	for name, p := range s.jvmHeapPools {
		used := sum(p.Used...)
		max := sum(p.Max...)
		peakUsed := sum(p.PeakUsed...)
		peakMax := sum(p.PeakMax...)

		name = strings.Title(name)
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
				Value:             aws.Float64(used / maxJVMHeapBytes * 100), // CloudWatch percents are int 0-100
			},
		)
	}

	// Sum garbage collection stats.
	for name, c := range s.garbageCollectors {
		count := sum(c.Count...)
		time := sum(c.Time...)

		name = strings.Title(name)
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

	// Sum filesystem utilization stats.
	if anyNonZero(s.maxFSBytes...) {
		sumMaxFSBytes := sum(s.maxFSBytes...)
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

// PushCloudwatchData pushes metrics to CloudWatch.
// The CloudWatch API has the following limitations:
//  - Max 40kb request size
//	- Single namespace per request
//	- Max 10 dimensions per metric
// Send metrics compressed and in batches.
func PushCloudwatchData(ctx context.Context, svc cloudwatchiface.CloudWatchAPI, data []*cloudwatch.MetricDatum) error {
	const batchSize = 30 // This is probably small enough.

	for i := 0; i < len(data); i += batchSize {
		j := i + batchSize
		if j > len(data) {
			j = len(data)
		}
		batch := data[i:j]
		req, _ := svc.PutMetricDataRequest(&cloudwatch.PutMetricDataInput{
			Namespace:  namespace,
			MetricData: batch,
		})
		req.Handlers.Build.PushBack(compressPayload)
		req.SetContext(ctx)
		if err := req.Send(); err != nil {
			return err
		}
		PushMetricsTotal.Add(float64(len(batch)))
	}
	return nil
}

// compressPayload compresses the payload before sending it to the API.
// According to the documentation:
// "Each PutMetricData request is limited to 40 KB in size for HTTP POST requests.
// You can send a payload compressed by gzip."
// src: https://github.com/cloudposse/prometheus-to-cloudwatch/blob/master/prometheus_to_cloudwatch.go#L237
func compressPayload(r *request.Request) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := io.Copy(zw, r.GetBody()); err != nil {
		zap.L().DPanic("error compressing CloudwatchRequest body", zap.Error(err))
		return
	}
	if err := zw.Close(); err != nil {
		zap.L().DPanic("error compressing CloudwatchRequest body", zap.Error(err))
		return
	}
	r.SetBufferBody(buf.Bytes())
	r.HTTPRequest.Header.Set("Content-Encoding", "gzip")
}

// LogDatum logs a CloudWatch data point at debug level.
func LogDatum(logger *zap.Logger, datum *cloudwatch.MetricDatum) {
	logger = logger.With(zap.String("name", *datum.MetricName), zap.Float64("value", *datum.Value), zap.Namespace("dimensions"))
	for _, d := range datum.Dimensions {
		logger = logger.With(zap.String(*d.Name, *d.Value))
	}
	logger.Debug("metric datum")
}

// sum returns a sum of d.
func sum(d ...float64) float64 {
	var sum float64
	for _, v := range d {
		sum += v
	}
	return sum
}

// countTrue returns a count of the number of true's in b.
func countTrue(b ...bool) int {
	sum := 0
	for _, v := range b {
		if v {
			sum++
		}
	}
	return sum
}

// anyNonZero returns true if any fs is non-zero.
func anyNonZero(fs ...float64) bool {
	for _, f := range fs {
		if f != 0 {
			return true
		}
	}
	return false
}
