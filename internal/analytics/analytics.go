// Package analytics contains pure, dependency-free statistics helpers used by
// the anomaly-detection stage. Keeping these free of DB/Watermill concerns makes
// them trivially unit-testable in isolation.
package analytics

import "math"

// ZScore computes the population mean and standard deviation of values and the
// z-score of latest relative to that distribution.
//
// It returns ok=false when there are too few points to form a distribution
// (fewer than two) or when the standard deviation is zero (every value equal),
// in which case a z-score is undefined. Callers should treat ok=false as "no
// signal" rather than an error.
func ZScore(values []float64, latest float64) (mean, stddev, z float64, ok bool) {
	if len(values) < 2 {
		return 0, 0, 0, false
	}

	var sum float64
	for _, v := range values {
		sum += v
	}
	mean = sum / float64(len(values))

	var sumSq float64
	for _, v := range values {
		d := v - mean
		sumSq += d * d
	}
	stddev = math.Sqrt(sumSq / float64(len(values)))

	if stddev == 0 {
		return mean, stddev, 0, false
	}

	z = (latest - mean) / stddev
	return mean, stddev, z, true
}
