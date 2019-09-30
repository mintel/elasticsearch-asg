package cloudwatcher

import (
	"testing"

	"github.com/stretchr/testify/assert" // Test assertions e.g. equality.

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

func TestNewEC2Instance(t *testing.T) {
	type args struct {
		i ec2.Instance
	}
	tests := []struct {
		name string
		args args
		want *EC2Instance
	}{
		{
			name: "basic",
			args: args{
				i: ec2.Instance{
					InstanceId: aws.String("i-123456789abc"),
					CpuOptions: &ec2.CpuOptions{
						CoreCount:      aws.Int64(4),
						ThreadsPerCore: aws.Int64(2),
					},
				},
			},
			want: &EC2Instance{
				ID:    "i-123456789abc",
				VCPUs: 8,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewEC2Instance(tt.args.i)
			assert.Equal(t, tt.want, got)
		})
	}
}
