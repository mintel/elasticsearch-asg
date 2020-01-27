package time

import (
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBetween(t *testing.T) {
	type args struct {
		t     time.Time
		start time.Time
		end   time.Time
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "between",
			args: args{
				start: time.Date(2019, time.June, 1, 0, 0, 0, 0, time.UTC),
				t:     time.Date(2019, time.June, 15, 0, 0, 0, 0, time.UTC),
				end:   time.Date(2019, time.July, 1, 0, 0, 0, 0, time.UTC),
			},
			want: true,
		},
		{
			name: "before",
			args: args{
				start: time.Date(2019, time.June, 1, 0, 0, 0, 0, time.UTC),
				t:     time.Date(2019, time.May, 20, 0, 0, 0, 0, time.UTC),
				end:   time.Date(2019, time.July, 1, 0, 0, 0, 0, time.UTC),
			},
			want: false,
		},
		{
			name: "after",
			args: args{
				start: time.Date(2019, time.June, 1, 0, 0, 0, 0, time.UTC),
				t:     time.Date(2019, time.July, 15, 0, 0, 0, 0, time.UTC),
				end:   time.Date(2019, time.July, 1, 0, 0, 0, 0, time.UTC),
			},
			want: false,
		},
		{
			name: "inclusive_left",
			args: args{
				start: time.Date(2019, time.June, 1, 0, 0, 0, 0, time.UTC),
				t:     time.Date(2019, time.June, 1, 0, 0, 0, 0, time.UTC),
				end:   time.Date(2019, time.July, 1, 0, 0, 0, 0, time.UTC),
			},
			want: true,
		},
		{
			name: "inclusive_right",
			args: args{
				start: time.Date(2019, time.June, 1, 0, 0, 0, 0, time.UTC),
				t:     time.Date(2019, time.July, 1, 0, 0, 0, 0, time.UTC),
				end:   time.Date(2019, time.July, 1, 0, 0, 0, 0, time.UTC),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Between(tt.args.t, tt.args.start, tt.args.end)
			assert.Equal(t, tt.want, got,
				"Between(%s, %s, %s) = %v, want %v", tt.args.t, tt.args.start, tt.args.end, got, tt.want,
			)
		})
	}

	// Check that Between(t, a, b) == Between(t, b, a).
	t.Run("args-order", func(t *testing.T) {
		reversedBetween := func(t, a, b time.Time) bool {
			return Between(t, b, a)
		}

		err := quick.CheckEqual(Between, reversedBetween, &quick.Config{
			Values: func(args []reflect.Value, r *rand.Rand) {
				randomTime := func() time.Time {
					sec := r.Int63()
					if r.Float64() > 0.5 {
						sec = -sec
					}
					nsec := r.Int63()
					if r.Float64() > 0.5 {
						nsec = -nsec
					}
					return time.Unix(sec, nsec)
				}
				args[0] = reflect.ValueOf(randomTime()) // t
				args[1] = reflect.ValueOf(randomTime()) // a
				args[2] = reflect.ValueOf(randomTime()) // b
			},
		})
		assert.NoError(t, err)
	})
}
