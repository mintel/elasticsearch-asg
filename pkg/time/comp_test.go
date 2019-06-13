package time

import (
	"testing"
	"time"
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
			if got := Between(tt.args.t, tt.args.start, tt.args.end); got != tt.want {
				t.Errorf("Between() = %v, want %v", got, tt.want)
			}
		})
	}
}
