package stats

import (
	"math"
	"sort"
)

// Compute calculates the requested aggregations on the given values.
// Supported aggregations: median, mean, p95, min, max.
func Compute(values []float64, aggs []string) map[string]float64 {
	result := make(map[string]float64, len(aggs))

	if len(values) == 0 {
		for _, a := range aggs {
			result[a] = 0
		}
		return result
	}

	// Check if any aggregation needs sorted data.
	needsSorted := false
	for _, a := range aggs {
		if a == "median" || a == "p95" {
			needsSorted = true
			break
		}
	}

	var sorted []float64
	if needsSorted {
		sorted = make([]float64, len(values))
		copy(sorted, values)
		sort.Float64s(sorted)
	}

	for _, a := range aggs {
		switch a {
		case "median":
			result["median"] = medianSorted(sorted)
		case "mean":
			result["mean"] = Mean(values)
		case "p95":
			result["p95"] = p95Sorted(sorted)
		case "min":
			result["min"] = Min(values)
		case "max":
			result["max"] = Max(values)
		}
	}

	return result
}

// medianSorted returns the median of an already-sorted slice.
func medianSorted(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

// p95Sorted returns the 95th percentile of an already-sorted slice.
func p95Sorted(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	rank := math.Ceil(0.95 * float64(n))
	idx := max(int(rank)-1, 0)
	if idx >= n {
		idx = n - 1
	}
	return sorted[idx]
}

// Median returns the median of a slice of float64 values.
func Median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

// Mean returns the arithmetic mean of a slice of float64 values.
func Mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// P95 returns the 95th percentile using the nearest-rank method.
func P95(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	n := float64(len(sorted))
	rank := math.Ceil(0.95 * n)
	idx := max(int(rank)-1, 0)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// Min returns the minimum value in a slice.
func Min(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	min := values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
	}
	return min
}

// Max returns the maximum value in a slice.
func Max(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	max := values[0]
	for _, v := range values[1:] {
		if v > max {
			max = v
		}
	}
	return max
}
