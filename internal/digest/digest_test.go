package digest

import (
	"testing"
	"time"
)

func TestNextRun(t *testing.T) {
	d := &Digest{hour: 9}
	loc := time.UTC

	cases := []struct {
		name string
		from time.Time
		want time.Time
	}{
		{
			name: "before hour same day",
			from: time.Date(2026, 7, 19, 6, 30, 0, 0, loc),
			want: time.Date(2026, 7, 19, 9, 0, 0, 0, loc),
		},
		{
			name: "after hour rolls to next day",
			from: time.Date(2026, 7, 19, 12, 0, 0, 0, loc),
			want: time.Date(2026, 7, 20, 9, 0, 0, 0, loc),
		},
		{
			name: "exactly at hour rolls to next day",
			from: time.Date(2026, 7, 19, 9, 0, 0, 0, loc),
			want: time.Date(2026, 7, 20, 9, 0, 0, 0, loc),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := d.nextRun(tc.from); !got.Equal(tc.want) {
				t.Fatalf("nextRun(%s) = %s, want %s", tc.from, got, tc.want)
			}
		})
	}
}
