package gitlab

import (
	"reflect"
	"testing"
	"time"
)

func Test_parseTime(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want time.Time
	}{
		{
			name: "simple test",
			s:    "2023-10-19 11:50:00 UTC",
			want: time.Date(2023, 10, 19, 11, 50, 00, 0, time.UTC),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseTime(tt.s); !reflect.DeepEqual(got, &tt.want) {
				t.Errorf("parseTime() = %v, want %v", got, tt.want)
			}
		})
	}
}
