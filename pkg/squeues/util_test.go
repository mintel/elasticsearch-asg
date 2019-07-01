package squeues

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegion(t *testing.T) {
	testCases := []struct {
		desc, input, want string
		err               bool
	}{
		{
			desc:  "basic",
			input: "https://sqs.us-east-2.amazonaws.com/123456789012/foo",
			want:  "us-east-2",
			err:   false,
		},
		{
			desc:  "fifo",
			input: "https://sqs.us-east-2.amazonaws.com/123456789012/bar.fifo",
			want:  "us-east-2",
			err:   false,
		},
		{
			desc:  "invalid",
			input: "foobar",
			want:  "",
			err:   true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			got, err := Region(tC.input)
			if tC.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tC.want, got)
		})
	}
}
