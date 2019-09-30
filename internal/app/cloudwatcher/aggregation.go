package cloudwatcher

import (
	"gonum.org/v1/gonum/floats" // Float math tools.

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
)

// AggregationData represents a container for some data series
// about Elasticsearch nodes, that can be converted
// into a CloudWatch Metrics data point.
type AggregationData interface {
	// AddSample adds the given node to the data series
	AddSample(*NodeStats)

	// Datum returns the aggregated data as a CloudWatch
	// Metrics data point.
	Datum() cloudwatch.MetricDatum
}

// SelectorFn returns some aspect of a NodeStats to
// be aggregated. If the node shouldn't count towards
// a data set, the selector function should return nil.
type SelectorFn func(*NodeStats) *float64

// StatsData aggregates count, min, max, and sum of the samples.
type StatsData struct {
	Template cloudwatch.MetricDatum
	Selector SelectorFn

	data []float64
}

var _ AggregationData = (*StatsData)(nil)

func (d *StatsData) AddSample(ns *NodeStats) {
	if f := d.Selector(ns); f != nil {
		d.data = append(d.data, *f)
	}
}

func (d *StatsData) Datum() cloudwatch.MetricDatum {
	m := d.Template
	if len(d.data) != 0 {
		m.StatisticValues = &cloudwatch.StatisticSet{
			SampleCount: aws.Float64(float64(len(d.data))),
			Minimum:     aws.Float64(floats.Min(d.data)),
			Maximum:     aws.Float64(floats.Max(d.data)),
			Sum:         aws.Float64(floats.Sum(d.data)),
		}
	}
	return m
}

// SumData aggregates a simple sum of samples.
type SumData struct {
	Template cloudwatch.MetricDatum
	Selector SelectorFn

	data []float64
}

var _ AggregationData = (*SumData)(nil)

func (d *SumData) AddSample(ns *NodeStats) {
	if f := d.Selector(ns); f != nil {
		d.data = append(d.data, *f)
	}
}

func (d *SumData) Datum() cloudwatch.MetricDatum {
	m := d.Template
	if len(d.data) != 0 {
		m.Value = aws.Float64(floats.Sum(d.data))
	}
	return m
}

// UtilizationData aggregates a percentage representing the
// utilization of some resource based on numerator and denominator.
// For example, disk utilization == bytes used / bytes total.
type UtilizationData struct {
	Template    cloudwatch.MetricDatum
	Numerator   SelectorFn
	Denominator SelectorFn

	num   []float64
	denom []float64
}

var _ AggregationData = (*UtilizationData)(nil)

func (d *UtilizationData) AddSample(ns *NodeStats) {
	num, denom := d.Numerator(ns), d.Denominator(ns)
	if num != nil && denom != nil {
		d.num = append(d.num, *num)
		d.denom = append(d.denom, *denom)
	}
}

func (d *UtilizationData) Datum() cloudwatch.MetricDatum {
	m := d.Template
	num, denom := floats.Sum(d.num), floats.Sum(d.denom)
	if denom != 0 {
		m.Value = aws.Float64((num / denom) * 100) // CloudWatch percents are int 0-100.
	}
	return m
}
