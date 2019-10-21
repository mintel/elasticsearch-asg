package events_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert" // Test assertions e.g. equality.

	"github.com/mintel/elasticsearch-asg/pkg/events" // AWS CloudWatch Events.
)

func Example() {
	msg := `{
		"version": "0",
		"id": "12345678-1234-1234-1234-123456789012",
		"detail-type": "EC2 Spot Instance Interruption Warning",
		"source": "aws.ec2",
		"account": "123456789012",
		"time": "2019-09-26T12:55:24Z",
		"region": "us-east-2",
		"resources": ["arn:aws:ec2:us-east-2:123456789012:instance/i-1234567890abcdef0"],
		"detail": {
			"instance-id": "i-1234567890abcdef0",
			"instance-action": "terminate"
		}
	}`

	e := &events.CloudWatchEvent{}
	if err := json.Unmarshal([]byte(msg), e); err != nil {
		panic(err)
	}

	fmt.Println(e.Detail.(*events.EC2SpotInterruption).InstanceID)
	// Output: i-1234567890abcdef0
}

func ExampleRegisterDetailType() {
	// MyDetailType is a custom CloudWatch Event type.
	type MyDetailType struct {
		Key1 string `json:"key1"`
		Key2 string `json:"key2"`
	}

	events.MustRegisterDetailType(
		"com.mycompany.myapp",
		"myDetailType",
		reflect.TypeOf(MyDetailType{}),
	)

	data := []byte(`{
		"version": "0",
		"id": "12345678-1234-1234-1234-123456789012",
		"detail-type": "myDetailType",
		"source": "com.mycompany.myapp",
		"account": "123456789012",
		"time": "2019-09-26T12:55:24Z",
		"region": "us-east-2",
		"resources": ["resource1", "resource2"],
		"detail": {
			"key1": "value1",
			"key2": "value2"
		}
	}`)

	v := &events.CloudWatchEvent{}
	if err := json.Unmarshal(data, v); err != nil {
		panic(err)
	}
	details := v.Detail.(*MyDetailType)
	fmt.Println(details.Key1)
	fmt.Println(details.Key2)
	// Output:
	// value1
	// value2
}

func TestCloudWatchEvent_Unmarshal(t *testing.T) {
	t.Run("unknown-detail-type", func(t *testing.T) {
		data := []byte(`{
			"version": "0",
			"id": "12345678-1234-1234-1234-123456789012",
			"detail-type": "Unknown Detail Type",
			"source": "aws.foobar",
			"account": "123456789012",
			"time": "2019-09-26T12:55:24Z",
			"region": "us-east-2",
			"resources": ["arn:aws:ec2:us-east-2:123456789012:instance/i-1234567890abcdef0"],
			"detail": {
				"foobar-instance": "i-1234567890abcdef0"
			}
		}`)
		v := &events.CloudWatchEvent{}
		err := json.Unmarshal(data, v)
		assert.NoError(t, err)

		// If the detail type is unknown, should unmarshal to
		// a generic map.
		wantDetail := map[string]interface{}{
			"foobar-instance": "i-1234567890abcdef0",
		}
		assert.Equal(t, wantDetail, v.Detail)
	})
}
