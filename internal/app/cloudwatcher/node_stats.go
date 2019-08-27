package cloudwatcher

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	elastic "github.com/olivere/elastic/v7"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"

	"github.com/mintel/elasticsearch-asg/pkg/es"
	"github.com/mintel/elasticsearch-asg/pkg/str"
)

var (
	// errInconsistentNodes is returns by newNodeStats when
	// the inputs refer to different Elasticsearch nodes.
	errInconsistentNodes = errors.New("inconsistent nodes")
)

// JVMHeapStats represents stats about the JVM
// heap of an Elasticsearch node.
type JVMHeapStats struct {
	MaxBytes  int64
	UsedBytes int64
}

// NodeStats represents stats about an Elasticsearch node.
type NodeStats struct {
	Name  string
	Roles []string

	// Node is excluded from shard allocation.
	ExcludedFromAllocation bool

	// Count of vCPUs.
	VCPUs int

	// Linux load.
	Load1m  float64
	Load5m  float64
	Load15m float64

	// JVM heap stats.
	JVMHeapStats JVMHeapStats
	JVMHeapPools map[string]JVMHeapStats

	// Elasticsearch disk size in bytes.
	FilesystemTotalBytes int64

	// Elasticsearch disk available bytes.
	FilesystemAvailableBytes int64
}

// NewNodeStats returns a new nodeStats based on the
// responses from various APIs. It returns
// ErrInconsistentNodes if the responses are for different
// nodes.
func NewNodeStats(
	s *elastic.NodesStatsNode,
	i *EC2Instance,
	transient *es.ShardAllocationExcludeSettings,
	persistent *es.ShardAllocationExcludeSettings,
) (*NodeStats, error) {
	if s.Name != i.ID {
		return nil, errInconsistentNodes
	}

	excluded := (str.In(s.Name, transient.Name...) ||
		str.In(s.IP, transient.IP...) ||
		str.In(s.Host, transient.Host...) ||
		str.In(s.Name, persistent.Name...) ||
		str.In(s.IP, persistent.IP...) ||
		str.In(s.Host, persistent.Host...))
	if !excluded {
		for k, v := range s.Attributes {
			if sv, ok := transient.Attr[k]; ok && str.In(fmt.Sprint(v), sv...) {
				excluded = true
				break
			}
			if sv, ok := persistent.Attr[k]; ok && str.In(fmt.Sprint(v), sv...) {
				excluded = true
				break
			}
		}
	}

	roles := append([]string(nil), s.Roles...)
	sort.Strings(roles)

	n := &NodeStats{
		Name:                   s.Name,
		Roles:                  roles,
		ExcludedFromAllocation: excluded,
		VCPUs:                  i.VCPUs,
		JVMHeapStats: JVMHeapStats{
			MaxBytes:  s.JVM.Mem.HeapMaxInBytes,
			UsedBytes: s.JVM.Mem.HeapUsedInBytes,
		},
		JVMHeapPools:             make(map[string]JVMHeapStats, len(s.JVM.Mem.Pools)),
		Load1m:                   s.OS.CPU.LoadAverage["1m"],
		Load5m:                   s.OS.CPU.LoadAverage["5m"],
		Load15m:                  s.OS.CPU.LoadAverage["15m"],
		FilesystemTotalBytes:     s.FS.Total.TotalInBytes,
		FilesystemAvailableBytes: s.FS.Total.AvailableInBytes,
	}
	for k, v := range s.JVM.Mem.Pools {
		n.JVMHeapPools[k] = JVMHeapStats{
			MaxBytes:  v.MaxInBytes,
			UsedBytes: v.UsedInBytes,
		}
	}

	return n, nil
}

// HasRole returns true if the node has a particular role.
func (s *NodeStats) HasRole(role string) bool {
	switch role {
	case "all":
		return true
	case "coordinate":
		return len(s.Roles) == 0
	}
	i := sort.SearchStrings(s.Roles, role)
	return i < len(s.Roles) && s.Roles[i] == role
}

// NodeStatsSlice is a slice of nodeStats.
type NodeStatsSlice []*NodeStats

// Aggregate the nodeStats of this slice into a set
// of CloudWatch metric data points.
//
// Metrics include:
//
// - File system utilization (data nodes only)
// - JVM heap utilization (both in total, and per-memory pool)
// - Linux Load
// - Count of nodes excluded from shard allocation
//
func (s NodeStatsSlice) Aggregate(dimensions []cloudwatch.Dimension) []cloudwatch.MetricDatum {
	if len(s) == 0 {
		return nil
	}
	now := time.Now()

	aggs := []AggregationData{
		&SumData{
			Template: cloudwatch.MetricDatum{
				MetricName:        aws.String("CountNodes"),
				Timestamp:         aws.Time(now),
				Dimensions:        dimensions,
				StorageResolution: aws.Int64(1),
				Unit:              cloudwatch.StandardUnitCount,
			},
			Selector: func(ns *NodeStats) *float64 {
				f := float64(1)
				return &f
			},
		},

		&SumData{
			Template: cloudwatch.MetricDatum{
				MetricName:        aws.String("CountvCPU"),
				Timestamp:         aws.Time(now),
				Dimensions:        dimensions,
				StorageResolution: aws.Int64(1),
				Unit:              cloudwatch.StandardUnitCount,
			},
			Selector: func(ns *NodeStats) *float64 {
				f := float64(ns.VCPUs)
				return &f
			},
		},

		&StatsData{
			Template: cloudwatch.MetricDatum{
				MetricName:        aws.String("Load1m"),
				Timestamp:         aws.Time(now),
				Dimensions:        dimensions,
				StorageResolution: aws.Int64(1),
				Unit:              cloudwatch.StandardUnitCount,
			},
			Selector: func(ns *NodeStats) *float64 {
				return &ns.Load1m
			},
		},

		&StatsData{
			Template: cloudwatch.MetricDatum{
				MetricName:        aws.String("Load5m"),
				Timestamp:         aws.Time(now),
				Dimensions:        dimensions,
				StorageResolution: aws.Int64(1),
				Unit:              cloudwatch.StandardUnitCount,
			},
			Selector: func(ns *NodeStats) *float64 {
				return &ns.Load5m
			},
		},

		&StatsData{
			Template: cloudwatch.MetricDatum{
				MetricName:        aws.String("Load15m"),
				Timestamp:         aws.Time(now),
				Dimensions:        dimensions,
				StorageResolution: aws.Int64(1),
				Unit:              cloudwatch.StandardUnitCount,
			},
			Selector: func(ns *NodeStats) *float64 {
				return &ns.Load15m
			},
		},

		&UtilizationData{
			Template: cloudwatch.MetricDatum{
				MetricName:        aws.String("Load1mUtilization"),
				Timestamp:         aws.Time(now),
				Dimensions:        dimensions,
				StorageResolution: aws.Int64(1),
				Unit:              cloudwatch.StandardUnitNone,
			},
			Numerator: func(ns *NodeStats) *float64 {
				return &ns.Load1m
			},
			Denominator: func(ns *NodeStats) *float64 {
				f := float64(ns.VCPUs)
				return &f
			},
		},

		&UtilizationData{
			Template: cloudwatch.MetricDatum{
				MetricName:        aws.String("Load5mUtilization"),
				Timestamp:         aws.Time(now),
				Dimensions:        dimensions,
				StorageResolution: aws.Int64(1),
				Unit:              cloudwatch.StandardUnitNone,
			},
			Numerator: func(ns *NodeStats) *float64 {
				return &ns.Load5m
			},
			Denominator: func(ns *NodeStats) *float64 {
				f := float64(ns.VCPUs)
				return &f
			},
		},

		&UtilizationData{
			Template: cloudwatch.MetricDatum{
				MetricName:        aws.String("Load15mUtilization"),
				Timestamp:         aws.Time(now),
				Dimensions:        dimensions,
				StorageResolution: aws.Int64(1),
				Unit:              cloudwatch.StandardUnitNone,
			},
			Numerator: func(ns *NodeStats) *float64 {
				return &ns.Load15m
			},
			Denominator: func(ns *NodeStats) *float64 {
				f := float64(ns.VCPUs)
				return &f
			},
		},

		&SumData{
			Template: cloudwatch.MetricDatum{
				MetricName:        aws.String("CountExcludedFromAllocation"),
				Timestamp:         aws.Time(now),
				Dimensions:        dimensions,
				StorageResolution: aws.Int64(1),
				Unit:              cloudwatch.StandardUnitCount,
			},
			Selector: func(ns *NodeStats) *float64 {
				var f float64
				if ns.ExcludedFromAllocation {
					f = 1
				}
				return &f
			},
		},

		&StatsData{
			Template: cloudwatch.MetricDatum{
				MetricName:        aws.String("JVMMaxBytes"),
				Timestamp:         aws.Time(now),
				Dimensions:        dimensions,
				StorageResolution: aws.Int64(1),
				Unit:              cloudwatch.StandardUnitBytes,
			},
			Selector: func(ns *NodeStats) *float64 {
				f := float64(ns.JVMHeapStats.MaxBytes)
				return &f
			},
		},

		&StatsData{
			Template: cloudwatch.MetricDatum{
				MetricName:        aws.String("JVMUsedBytes"),
				Timestamp:         aws.Time(now),
				Dimensions:        dimensions,
				StorageResolution: aws.Int64(1),
				Unit:              cloudwatch.StandardUnitBytes,
			},
			Selector: func(ns *NodeStats) *float64 {
				f := float64(ns.JVMHeapStats.UsedBytes)
				return &f
			},
		},

		&UtilizationData{
			Template: cloudwatch.MetricDatum{
				MetricName:        aws.String("JVMUtilization"),
				Timestamp:         aws.Time(now),
				Dimensions:        dimensions,
				StorageResolution: aws.Int64(1),
				Unit:              cloudwatch.StandardUnitPercent,
			},
			Numerator: func(ns *NodeStats) *float64 {
				f := float64(ns.JVMHeapStats.UsedBytes)
				return &f
			},
			Denominator: func(ns *NodeStats) *float64 {
				f := float64(ns.JVMHeapStats.MaxBytes)
				return &f
			},
		},

		&StatsData{
			Template: cloudwatch.MetricDatum{
				MetricName:        aws.String("FSTotalBytes"),
				Timestamp:         aws.Time(now),
				Dimensions:        dimensions,
				StorageResolution: aws.Int64(1),
				Unit:              cloudwatch.StandardUnitBytes,
			},
			Selector: func(ns *NodeStats) *float64 {
				if !ns.HasRole("data") {
					return nil
				}
				f := float64(ns.FilesystemTotalBytes)
				return &f
			},
		},

		&StatsData{
			Template: cloudwatch.MetricDatum{
				MetricName:        aws.String("FSAvailableBytes"),
				Timestamp:         aws.Time(now),
				Dimensions:        dimensions,
				StorageResolution: aws.Int64(1),
				Unit:              cloudwatch.StandardUnitBytes,
			},
			Selector: func(ns *NodeStats) *float64 {
				if !ns.HasRole("data") {
					return nil
				}
				f := float64(ns.FilesystemAvailableBytes)
				return &f
			},
		},

		&UtilizationData{
			Template: cloudwatch.MetricDatum{
				MetricName:        aws.String("FSUtilization"),
				Timestamp:         aws.Time(now),
				Dimensions:        dimensions,
				StorageResolution: aws.Int64(1),
				Unit:              cloudwatch.StandardUnitPercent,
			},
			Numerator: func(ns *NodeStats) *float64 {
				if !ns.HasRole("data") {
					return nil
				}
				f := float64(ns.FilesystemTotalBytes) - float64(ns.FilesystemAvailableBytes)
				return &f
			},
			Denominator: func(ns *NodeStats) *float64 {
				if !ns.HasRole("data") {
					return nil
				}
				f := float64(ns.FilesystemTotalBytes)
				return &f
			},
		},
	}

	pools := make(map[string]struct{}, 3)
	for _, ns := range s {
		for pool := range ns.JVMHeapPools {
			if _, ok := pools[pool]; !ok {
				func(pool string) { // Make sure refernce to pool isn't shadowed.
					name := strings.Title(pool)
					aggs = append(aggs,
						&StatsData{
							Template: cloudwatch.MetricDatum{
								MetricName:        aws.String(fmt.Sprintf("JVM%sPoolMaxBytes", name)),
								Timestamp:         aws.Time(now),
								Dimensions:        dimensions,
								StorageResolution: aws.Int64(1),
								Unit:              cloudwatch.StandardUnitBytes,
							},
							Selector: func(ns *NodeStats) *float64 {
								f := float64(ns.JVMHeapPools[pool].MaxBytes)
								return &f
							},
						},

						&StatsData{
							Template: cloudwatch.MetricDatum{
								MetricName:        aws.String(fmt.Sprintf("JVM%sPoolUsedBytes", name)),
								Timestamp:         aws.Time(now),
								Dimensions:        dimensions,
								StorageResolution: aws.Int64(1),
								Unit:              cloudwatch.StandardUnitBytes,
							},
							Selector: func(ns *NodeStats) *float64 {
								f := float64(ns.JVMHeapPools[pool].UsedBytes)
								return &f
							},
						},

						&UtilizationData{
							Template: cloudwatch.MetricDatum{
								MetricName:        aws.String(fmt.Sprintf("JVM%sPoolUtilization", name)),
								Timestamp:         aws.Time(now),
								Dimensions:        dimensions,
								StorageResolution: aws.Int64(1),
								Unit:              cloudwatch.StandardUnitPercent,
							},
							Numerator: func(ns *NodeStats) *float64 {
								f := float64(ns.JVMHeapPools[pool].UsedBytes)
								return &f
							},
							Denominator: func(ns *NodeStats) *float64 {
								f := float64(ns.JVMHeapPools[pool].MaxBytes)
								return &f
							},
						},
					)
				}(pool)
				pools[pool] = struct{}{}
			}
		}

		for _, o := range aggs {
			o.AddSample(ns)
		}
	}

	out := make([]cloudwatch.MetricDatum, len(aggs))
	i := 0
	for _, a := range aggs {
		m := a.Datum()
		if m.Value != nil || m.StatisticValues != nil || m.Values != nil {
			out[i] = m
			i++
		}
	}

	return out
}
