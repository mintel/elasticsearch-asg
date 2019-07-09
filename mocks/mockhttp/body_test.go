package mockhttp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type Body struct {
	X int    `json:"x"`
	Y string `json:"y"`

	z bool // An unexported field. Shouldn't effect mock assertions.
}

func TestBodyBytes(t *testing.T) {
	testCases := []struct {
		desc string
		body interface{}
		want []byte
	}{
		{
			desc: "string",
			body: "foobar",
			want: []byte("foobar"),
		},
		{
			desc: "bytes",
			body: []byte("foobar"),
			want: []byte("foobar"),
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			if got, err := bodyBytes(tC.body); assert.NoError(t, err) {
				assert.Equal(t, tC.want, got)
			}
		})
	}
}

func TestBodiesEqual(t *testing.T) {
	testCases := []struct {
		desc          string
		a, b          interface{}
		want, wantErr bool
	}{
		{
			desc: "strings",
			a:    "foobar",
			b:    "foobar",
			want: true,
		},
		{
			desc: "not-equal",
			a:    "foo",
			b:    "bar",
			want: false,
		},
		{
			desc: "bytes-string",
			a:    []byte(`foobar`),
			b:    "foobar",
			want: true,
		},
		{
			desc: "bytes-struct",
			a:    []byte(`{"x": 1, "y": "foobar"}`),
			b: &Body{
				X: 1,
				Y: "foobar",
				z: true, // Shouldn't effect equality.
			},
			want: true,
		},
		{
			desc: "bytes-struct-reverse",
			a:    []byte(`{"y": "foobar", "x": 1}`),
			b: &Body{
				X: 1,
				Y: "foobar",
				z: true, // Shouldn't effect equality.
			},
			want: true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			got, err := bodiesEqual(tC.a, tC.b)
			if tC.wantErr {
				assert.False(t, got)
				assert.Error(t, err)
			} else {
				assert.Equal(t, tC.want, got)
				assert.NoError(t, err)
			}
		})
	}
}
