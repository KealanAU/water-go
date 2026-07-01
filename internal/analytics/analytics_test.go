package analytics

import (
	"math"
	"testing"
)

func TestZScore(t *testing.T) {
	tests := []struct {
		name     string
		values   []float64
		latest   float64
		wantMean float64
		wantStd  float64
		wantZ    float64
		wantOK   bool
	}{
		{
			name:   "too few points",
			values: []float64{5},
			latest: 5,
			wantOK: false,
		},
		{
			name:   "empty",
			values: nil,
			latest: 1,
			wantOK: false,
		},
		{
			name:   "zero stddev",
			values: []float64{3, 3, 3, 3},
			latest: 9,
			wantOK: false,
		},
		{
			name:     "basic distribution",
			values:   []float64{2, 4, 4, 4, 5, 5, 7, 9},
			latest:   10,
			wantMean: 5,
			wantStd:  2,
			wantZ:    2.5,
			wantOK:   true,
		},
		{
			name:     "latest below mean",
			values:   []float64{0, 10},
			latest:   0,
			wantMean: 5,
			wantStd:  5,
			wantZ:    -1,
			wantOK:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mean, std, z, ok := ZScore(tt.values, tt.latest)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if !approx(mean, tt.wantMean) {
				t.Errorf("mean = %v, want %v", mean, tt.wantMean)
			}
			if !approx(std, tt.wantStd) {
				t.Errorf("stddev = %v, want %v", std, tt.wantStd)
			}
			if !approx(z, tt.wantZ) {
				t.Errorf("z = %v, want %v", z, tt.wantZ)
			}
		})
	}
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
