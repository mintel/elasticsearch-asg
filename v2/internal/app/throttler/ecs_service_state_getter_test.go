package throttler

import (
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/mintel/elasticsearch-asg/v2/internal/app/throttler/mocks"
	"github.com/stretchr/testify/assert" // Test assertions e.g. equality.
	"testing"
)

func TestECSStateGetter(t *testing.T) {

	t.Run("singleDeployment", func(t *testing.T) {
		clusterName := "MyECSCluser"
		serviceName := "MyECSService"

		// So, the client.On lets us mock response behavior
		// But what does the client setup actually achieve on its own?
		// It returns the Request ...
		client := &mocks.ECS{}
		client.Test(t)
		client.On("DescribeServicesRequest", &ecs.DescribeServicesInput{
			Cluster:  &clusterName,
			Services: []string{serviceName},
		}).Return(
			&ecs.DescribeServicesOutput{
				Services: []ecs.Service{
					ecs.Service{
						Deployments: []ecs.Deployment{
							ecs.Deployment{},
						},
					},
				},
			},
			error(nil),
		).Once()

		ecsState, err := MockECSServiceStateGetter(client).Get(clusterName, serviceName)
		if assert.NoError(t, err) {
			assert.Equal(t, ecsState.NumDeployments, 1)
		}

		client.AssertExpectations(t)
	})

	t.Run("multipleDeployment", func(t *testing.T) {
		clusterName := "MyECSCluser"
		serviceName := "MyECSService"

		// So, the client.On lets us mock response behavior
		// But what does the client setup actually achieve on its own?
		// It returns the Request ...
		client := &mocks.ECS{}
		client.Test(t)
		client.On("DescribeServicesRequest", &ecs.DescribeServicesInput{
			Cluster:  &clusterName,
			Services: []string{serviceName},
		}).Return(
			&ecs.DescribeServicesOutput{
				Services: []ecs.Service{
					ecs.Service{
						Deployments: []ecs.Deployment{
							ecs.Deployment{},
							ecs.Deployment{},
						},
					},
				},
			},
			error(nil),
		).Once()

		ecsState, err := MockECSServiceStateGetter(client).Get(clusterName, serviceName)
		if assert.NoError(t, err) {
			assert.Equal(t, ecsState.NumDeployments, 2)
		}

		client.AssertExpectations(t)
	})
}
