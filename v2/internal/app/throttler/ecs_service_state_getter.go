package throttler

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/ecsiface"
	"github.com/pkg/errors"
)

// ECSServiceState represents the state of an AWS ECS
// Service in the context of the throttler app.
type ECSServiceState struct {
	// The number of deployments the service has
	// going on. The normal number of deployments
	// is 1: the currently running version of the service.
	// If there are > 1, that probably means a new version
	// of the service is being deployed.
	NumDeployments int
}

// ElasticsearchStateGetter queries AWS to return status information
// about an ECS service in an ECS cluster that is useful when deciding
// whether to allow scaling up or down of the Elasticsearch cluster.
type ECSServiceStateGetter struct {
	client ecsiface.ClientAPI
}

// NewECSServiceStateGetter returns a new ECSServiceStateGetter.
func NewECSServiceStateGetter(cfg aws.Config) *ECSServiceStateGetter {
	return &ECSServiceStateGetter{
		client: ecs.New(cfg),
	}
}

// MockECSServiceStateGetter takes an ECSClient arg and returns a new ECSServiceStateGetter.
func MockECSServiceStateGetter(mockClient ecsiface.ClientAPI) *ECSServiceStateGetter {
	return &ECSServiceStateGetter{
		client: mockClient,
	}
}

// Get returns the state of an ECS service.
func (g *ECSServiceStateGetter) Get(cluster, service string) (*ECSServiceState, error) {
	req := g.client.DescribeServicesRequest(&ecs.DescribeServicesInput{
		Cluster:  aws.String(cluster),
		Services: []string{service},
	})
	resp, err := req.Send(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "error DescribeServices "+service)
	}
	serv := resp.DescribeServicesOutput.Services[0]
	state := &ECSServiceState{
		NumDeployments: len(serv.Deployments),
	}
	return state, nil
}
