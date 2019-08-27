package drainer

import (
	"context"
	"testing"
	"time"

	"github.com/olebedev/emitter"
	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/mintel/elasticsearch-asg/internal/app/drainer/mocks"
	"github.com/mintel/elasticsearch-asg/internal/pkg/testutil"
	"github.com/mintel/elasticsearch-asg/pkg/events"
)

func TestCloudWatchEventEmitter_Run(t *testing.T) {
	_, teardown := testutil.TestLogger(t)
	defer teardown()

	m := &mocks.SQS{}
	m.Test(t)
	const queueURL = "queue url"
	emt := emitter.New(2)
	e := NewCloudWatchEventEmitter(m, queueURL, emt)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	receiveInput := &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(queueURL),
		MaxNumberOfMessages: aws.Int64(10),
		WaitTimeSeconds:     aws.Int64(20),
	}

	m.On("ReceiveMessageRequest", receiveInput).
		Return(
			&sqs.ReceiveMessageOutput{
				Messages: []sqs.Message{
					sqs.Message{
						ReceiptHandle: aws.String("ed854ed0-da1f-464f-985a-d86f179f387f"),
						Body: aws.String(`{
					    "version": "0",
					    "id": "12345678-1234-1234-1234-123456789012",
					    "detail-type": "EC2 Spot Instance Interruption Warning",
					    "source": "aws.ec2",
					    "account": "123456789012",
					    "time": "2019-09-27T14:46:23Z",
					    "region": "us-east-2",
					    "resources": ["arn:aws:ec2:us-east-2:123456789012:instance/i-1234567890abcdef0"],
					    "detail": {
					        "instance-id": "i-1234567890abcdef0",
					        "instance-action": "terminate"
					    }
					}`),
					},
					sqs.Message{
						ReceiptHandle: aws.String("ebe90619-8333-4358-a273-646021b49983"),
						Body: aws.String(`{
						"version": "0",
						"id": "12345678-1234-1234-1234-123456789012",
						"detail-type": "EC2 Instance-terminate Lifecycle Action",
						"source": "aws.autoscaling",
						"account": "123456789012",
						"time": "2019-09-27T14:47:13Z",
						"region": "us-west-2",
						"resources": [
							"auto-scaling-group-arn"
						],
						"detail": {
							"LifecycleActionToken":"87654321-4321-4321-4321-210987654321",
							"AutoScalingGroupName":"my-asg",
							"LifecycleHookName":"my-lifecycle-hook",
							"EC2InstanceId":"i-1234567890abcdef0",
							"LifecycleTransition":"autoscaling:EC2_INSTANCE_TERMINATING",
							"NotificationMetadata":"additional-info"
						}
					}`),
					},
				},
			},
			error(nil),
		).
		Once()

	m.On("ReceiveMessageRequest", receiveInput).
		Return(&sqs.ReceiveMessageOutput{Messages: []sqs.Message{}}, error(nil)).
		Maybe()

	m.On("DeleteMessageBatchRequest", &sqs.DeleteMessageBatchInput{
		QueueUrl: aws.String(queueURL),
		Entries: []sqs.DeleteMessageBatchRequestEntry{
			sqs.DeleteMessageBatchRequestEntry{
				Id:            aws.String("0"),
				ReceiptHandle: aws.String("ed854ed0-da1f-464f-985a-d86f179f387f"),
			},
			sqs.DeleteMessageBatchRequestEntry{
				Id:            aws.String("1"),
				ReceiptHandle: aws.String("ebe90619-8333-4358-a273-646021b49983"),
			},
		},
	}).
		Return(&sqs.DeleteMessageBatchOutput{}, error(nil)).
		Once()

	spotEvents := emt.Once(topicKey("aws.ec2", "EC2 Spot Instance Interruption Warning"))
	lifecycleEvents := emt.Once(topicKey("aws.autoscaling", "EC2 Instance-terminate Lifecycle Action"))
	errc := make(chan error, 1)
	go func() {
		errc <- e.Run(ctx)
		close(errc)
	}()

	want := &events.CloudWatchEvent{
		Version:    "0",
		ID:         "12345678-1234-1234-1234-123456789012",
		DetailType: "EC2 Spot Instance Interruption Warning",
		Source:     "aws.ec2",
		AccountID:  "123456789012",
		Time:       time.Date(2019, time.September, 27, 14, 46, 23, 0, time.UTC),
		Region:     "us-east-2",
		Resources:  []string{"arn:aws:ec2:us-east-2:123456789012:instance/i-1234567890abcdef0"},
		Detail: &events.EC2SpotInterruption{
			InstanceID:     "i-1234567890abcdef0",
			InstanceAction: "terminate",
		},
	}
	got := (<-spotEvents).Args[0].(*events.CloudWatchEvent)
	assert.Equal(t, want, got)

	want = &events.CloudWatchEvent{
		Version:    "0",
		ID:         "12345678-1234-1234-1234-123456789012",
		DetailType: "EC2 Instance-terminate Lifecycle Action",
		Source:     "aws.autoscaling",
		AccountID:  "123456789012",
		Time:       time.Date(2019, time.September, 27, 14, 47, 13, 0, time.UTC),
		Region:     "us-west-2",
		Resources:  []string{"auto-scaling-group-arn"},
		Detail: &events.AutoScalingLifecycleTerminateAction{
			LifecycleActionToken: "87654321-4321-4321-4321-210987654321",
			AutoScalingGroupName: "my-asg",
			LifecycleHookName:    "my-lifecycle-hook",
			EC2InstanceID:        "i-1234567890abcdef0",
			LifecycleTransition:  "autoscaling:EC2_INSTANCE_TERMINATING",
			NotificationMetadata: "additional-info",
		},
	}
	got = (<-lifecycleEvents).Args[0].(*events.CloudWatchEvent)
	assert.Equal(t, want, got)

	cancel()
	err := <-errc
	assert.Equal(t, err, context.Canceled)

	m.AssertExpectations(t)
}
