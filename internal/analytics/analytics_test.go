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

func TestZScoreExactlyTwoPoints(t *testing.T) {
	mean, std, z, ok := ZScore([]float64{0, 10}, 20)
	if !ok {
		t.Fatal("two distinct points should yield a distribution")
	}
	if !approx(mean, 5) || !approx(std, 5) || !approx(z, 3) {
		t.Errorf("mean=%v std=%v z=%v; want 5,5,3", mean, std, z)
	}
}

func TestZScoreNegativeValues(t *testing.T) {
	mean, std, z, ok := ZScore([]float64{-4, -2, -2, -2, -1, -1, 1, 3}, -6)
	if !ok {
		t.Fatal("ok should be true")
	}
	// Symmetric to the "basic distribution" case but shifted/negated.
	if !approx(mean, -1) || !approx(std, 2) {
		t.Errorf("mean=%v std=%v; want -1,2", mean, std)
	}
	if z >= 0 {
		t.Errorf("z = %v; want negative", z)
	}
	if !approx(z, -2.5) {
		t.Errorf("z = %v; want -2.5", z)
	}
}

func TestZScoreLatestEqualsMeanIsZero(t *testing.T) {
	_, _, z, ok := ZScore([]float64{1, 3}, 2)
	if !ok {
		t.Fatal("ok should be true")
	}
	if !approx(z, 0) {
		t.Errorf("z = %v; want 0", z)
	}
}

func TestZScoreZeroOutputsWhenNotOK(t *testing.T) {
	mean, std, z, ok := ZScore(nil, 5)
	if ok || mean != 0 || std != 0 || z != 0 {
		t.Errorf("empty input should return zeroes and ok=false, got %v %v %v %v", mean, std, z, ok)
	}
}
