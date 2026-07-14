package handlers

import "testing"

func TestParseRangeHeader(t *testing.T) {
	cases := []struct {
		name          string
		header        string
		size          int64
		wantStart     int64
		wantEnd       int64
		wantRange     bool
		wantUnsatisfy bool
	}{
		{name: "absent header serves full", header: "", size: 10, wantRange: false},
		{name: "simple range", header: "bytes=2-6", size: 10, wantStart: 2, wantEnd: 6, wantRange: true},
		{name: "open ended", header: "bytes=500-", size: 1000, wantStart: 500, wantEnd: 999, wantRange: true},
		{name: "suffix", header: "bytes=-300", size: 1000, wantStart: 700, wantEnd: 999, wantRange: true},
		{name: "suffix larger than object clamps to full", header: "bytes=-5000", size: 1000, wantStart: 0, wantEnd: 999, wantRange: true},
		{name: "end clamped to size", header: "bytes=0-99999", size: 100, wantStart: 0, wantEnd: 99, wantRange: true},
		{name: "single byte", header: "bytes=0-0", size: 10, wantStart: 0, wantEnd: 0, wantRange: true},
		{name: "start beyond size is unsatisfiable", header: "bytes=100-", size: 100, wantUnsatisfy: true},
		{name: "suffix zero is unsatisfiable", header: "bytes=-0", size: 100, wantUnsatisfy: true},
		{name: "any range on empty object is unsatisfiable", header: "bytes=0-", size: 0, wantUnsatisfy: true},
		{name: "suffix on empty object is unsatisfiable", header: "bytes=-5", size: 0, wantUnsatisfy: true},
		{name: "multi range ignored", header: "bytes=0-1,3-4", size: 10, wantRange: false},
		{name: "non byte unit ignored", header: "items=0-5", size: 10, wantRange: false},
		{name: "end before start ignored", header: "bytes=6-2", size: 10, wantRange: false},
		{name: "garbage start ignored", header: "bytes=abc-5", size: 10, wantRange: false},
		{name: "garbage end ignored", header: "bytes=5-abc", size: 10, wantRange: false},
		{name: "garbage suffix ignored", header: "bytes=-abc", size: 10, wantRange: false},
		{name: "negative start ignored", header: "bytes=-5-8", size: 10, wantRange: false},
		{name: "missing dash ignored", header: "bytes=5", size: 10, wantRange: false},
		{name: "whitespace tolerated", header: "bytes= 2-6 ", size: 10, wantStart: 2, wantEnd: 6, wantRange: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rng, unsatisfiable := parseRangeHeader(tc.header, tc.size)
			if unsatisfiable != tc.wantUnsatisfy {
				t.Fatalf("unsatisfiable = %v, want %v", unsatisfiable, tc.wantUnsatisfy)
			}
			if tc.wantRange {
				if rng == nil {
					t.Fatalf("rng = nil, want %d-%d", tc.wantStart, tc.wantEnd)
				}
				if rng.start != tc.wantStart || rng.end != tc.wantEnd {
					t.Errorf("rng = %d-%d, want %d-%d", rng.start, rng.end, tc.wantStart, tc.wantEnd)
				}
			} else if rng != nil {
				t.Errorf("rng = %d-%d, want nil", rng.start, rng.end)
			}
		})
	}
}
