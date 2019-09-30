package cloudwatcher

import (
	"testing"

	"github.com/stretchr/testify/assert" // Test assertions e.g. equality.

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
)

func Test_statsData_AddSample(t *testing.T) {
	type fields struct {
		Selector SelectorFn
	}
	type args struct {
		ns *NodeStats
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *float64
	}{
		{
			name: "add",
			fields: fields{
				Selector: func(ns *NodeStats) *float64 {
					return &ns.Load1m
				},
			},
			args: args{
				ns: &NodeStats{
					Load1m: 10,
				},
			},
			want: aws.Float64(10),
		},
		{
			name: "do-not-add",
			fields: fields{
				Selector: func(ns *NodeStats) *float64 {
					return nil
				},
			},
			args: args{
				ns: &NodeStats{},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &StatsData{
				Selector: tt.fields.Selector,
			}
			assert.Empty(t, d.data)
			d.AddSample(tt.args.ns)
			if tt.want != nil {
				assert.Equal(t, []float64{*tt.want}, d.data)
			} else {
				assert.Empty(t, d.data)
			}
		})
	}
}

func Test_statsData_Datum(t *testing.T) {
	type fields struct {
		Template cloudwatch.MetricDatum
		data     []float64
	}
	tests := []struct {
		name   string
		fields fields
		want   cloudwatch.MetricDatum
	}{
		{
			name: "basic",
			fields: fields{
				Template: cloudwatch.MetricDatum{
					MetricName: aws.String("Foobar"),
				},
				data: []float64{1, 2, 3},
			},
			want: cloudwatch.MetricDatum{
				MetricName: aws.String("Foobar"),
				StatisticValues: &cloudwatch.StatisticSet{
					Maximum:     aws.Float64(3),
					Minimum:     aws.Float64(1),
					SampleCount: aws.Float64(3),
					Sum:         aws.Float64(6),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &StatsData{
				Template: tt.fields.Template,
				data:     tt.fields.data,
			}
			got := d.Datum()
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_sumData_AddSample(t *testing.T) {
	type fields struct {
		Selector SelectorFn
	}
	type args struct {
		ns *NodeStats
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *float64
	}{
		{
			name: "add",
			fields: fields{
				Selector: func(ns *NodeStats) *float64 {
					return &ns.Load1m
				},
			},
			args: args{
				ns: &NodeStats{
					Load1m: 10,
				},
			},
			want: aws.Float64(10),
		},
		{
			name: "do-not-add",
			fields: fields{
				Selector: func(ns *NodeStats) *float64 {
					return nil
				},
			},
			args: args{
				ns: &NodeStats{},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &SumData{
				Selector: tt.fields.Selector,
			}
			assert.Empty(t, d.data)
			d.AddSample(tt.args.ns)
			if tt.want != nil {
				assert.Equal(t, []float64{*tt.want}, d.data)
			} else {
				assert.Empty(t, d.data)
			}
		})
	}
}

func Test_sumData_Datum(t *testing.T) {
	type fields struct {
		Template cloudwatch.MetricDatum
		data     []float64
	}
	tests := []struct {
		name   string
		fields fields
		want   cloudwatch.MetricDatum
	}{
		{
			name: "basic",
			fields: fields{
				Template: cloudwatch.MetricDatum{
					MetricName: aws.String("Foobar"),
				},
				data: []float64{1, 2, 3},
			},
			want: cloudwatch.MetricDatum{
				MetricName: aws.String("Foobar"),
				Value:      aws.Float64(6),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &SumData{
				Template: tt.fields.Template,
				data:     tt.fields.data,
			}
			got := d.Datum()
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_utilizationData_AddSample(t *testing.T) {
	type fields struct {
		Numerator   SelectorFn
		Denominator SelectorFn
	}
	type args struct {
		ns *NodeStats
	}
	type want struct {
		num   float64
		denom float64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *want
	}{
		{
			name: "add",
			fields: fields{
				Numerator: func(ns *NodeStats) *float64 {
					return &ns.Load1m
				},
				Denominator: func(ns *NodeStats) *float64 {
					f := float64(ns.VCPUs)
					return &f
				},
			},
			args: args{
				ns: &NodeStats{
					Load1m: 10,
					VCPUs:  4,
				},
			},
			want: &want{
				num:   10,
				denom: 4,
			},
		},
		{
			name: "do-not-add-numerator",
			fields: fields{
				Numerator: func(ns *NodeStats) *float64 {
					return nil
				},
				Denominator: func(ns *NodeStats) *float64 {
					f := float64(ns.VCPUs)
					return &f
				},
			},
			args: args{
				ns: &NodeStats{
					Load1m: 10,
					VCPUs:  4,
				},
			},
			want: nil,
		},
		{
			name: "do-not-add-denominator",
			fields: fields{
				Numerator: func(ns *NodeStats) *float64 {
					return &ns.Load1m
				},
				Denominator: func(ns *NodeStats) *float64 {
					return nil
				},
			},
			args: args{
				ns: &NodeStats{
					Load1m: 10,
					VCPUs:  4,
				},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &UtilizationData{
				Numerator:   tt.fields.Numerator,
				Denominator: tt.fields.Denominator,
			}
			assert.Empty(t, d.num)
			assert.Empty(t, d.denom)
			d.AddSample(tt.args.ns)
			if tt.want != nil {
				assert.Equal(t, []float64{tt.want.num}, d.num)
				assert.Equal(t, []float64{tt.want.denom}, d.denom)
			} else {
				assert.Empty(t, d.num)
				assert.Empty(t, d.denom)
			}
		})
	}
}

func Test_utilizationData_Datum(t *testing.T) {
	type fields struct {
		Template cloudwatch.MetricDatum
		num      []float64
		denom    []float64
	}
	tests := []struct {
		name   string
		fields fields
		want   cloudwatch.MetricDatum
	}{
		{
			name: "basic",
			fields: fields{
				Template: cloudwatch.MetricDatum{
					MetricName: aws.String("Foobar"),
				},
				num:   []float64{1, 2, 3},
				denom: []float64{3, 3, 3},
			},
			want: cloudwatch.MetricDatum{
				MetricName: aws.String("Foobar"),
				Value:      aws.Float64((6.0 / 9.0) * 100),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &UtilizationData{
				Template: tt.fields.Template,
				num:      tt.fields.num,
				denom:    tt.fields.denom,
			}
			got := d.Datum()
			assert.InDelta(t, *tt.want.Value, *got.Value, delta)
			tt.want.Value = nil
			got.Value = nil
			assert.Equal(t, tt.want, got)
		})
	}
}
