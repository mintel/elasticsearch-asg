package cloudwatcher

import (
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// EC2Instance is simplified version of an
// ec2.Instance that can be cached.
type EC2Instance struct {
	ID    string
	VCPUs int
}

// NewEC2Instance returns a new EC2Instance.
func NewEC2Instance(i ec2.Instance) *EC2Instance {
	return &EC2Instance{
		ID:    *i.InstanceId,
		VCPUs: int((*i.CpuOptions.CoreCount) * (*i.CpuOptions.ThreadsPerCore)),
	}
}
