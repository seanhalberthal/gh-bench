package stats

import (
	"math"
	"testing"
)

const epsilon = 1e-9

func assertClose(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > epsilon {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestMedian_OddLength(t *testing.T) {
	assertClose(t, Median([]float64{3, 1, 2}), 2)
}

func TestMedian_EvenLength(t *testing.T) {
	assertClose(t, Median([]float64{4, 1, 3, 2}), 2.5)
}

func TestMedian_SingleValue(t *testing.T) {
	assertClose(t, Median([]float64{42}), 42)
}

func TestMedian_Empty(t *testing.T) {
	assertClose(t, Median(nil), 0)
}

func TestMedian_DoesNotMutateInput(t *testing.T) {
	input := []float64{3, 1, 2}
	Median(input)
	if input[0] != 3 || input[1] != 1 || input[2] != 2 {
		t.Error("Median mutated the input slice")
	}
}

func TestMean_Basic(t *testing.T) {
	assertClose(t, Mean([]float64{10, 20, 30}), 20)
}

func TestMean_SingleValue(t *testing.T) {
	assertClose(t, Mean([]float64{7}), 7)
}

func TestMean_Empty(t *testing.T) {
	assertClose(t, Mean(nil), 0)
}

func TestMean_NegativeNumbers(t *testing.T) {
	assertClose(t, Mean([]float64{-10, 10}), 0)
}

func TestP95_Basic(t *testing.T) {
	// 20 values: 1..20. p95 = ceil(0.95*20) = 19th value = 19
	values := make([]float64, 20)
	for i := range values {
		values[i] = float64(i + 1)
	}
	assertClose(t, P95(values), 19)
}

func TestP95_SingleValue(t *testing.T) {
	assertClose(t, P95([]float64{5}), 5)
}

func TestP95_Empty(t *testing.T) {
	assertClose(t, P95(nil), 0)
}

func TestP95_TwoValues(t *testing.T) {
	// ceil(0.95*2) = 2, index 1 = second value
	assertClose(t, P95([]float64{1, 100}), 100)
}

func TestMin_Basic(t *testing.T) {
	assertClose(t, Min([]float64{5, 3, 8, 1, 9}), 1)
}

func TestMin_SingleValue(t *testing.T) {
	assertClose(t, Min([]float64{42}), 42)
}

func TestMin_Empty(t *testing.T) {
	assertClose(t, Min(nil), 0)
}

func TestMin_NegativeNumbers(t *testing.T) {
	assertClose(t, Min([]float64{-5, -1, -10}), -10)
}

func TestMin_IdenticalValues(t *testing.T) {
	assertClose(t, Min([]float64{7, 7, 7}), 7)
}

func TestMax_Basic(t *testing.T) {
	assertClose(t, Max([]float64{5, 3, 8, 1, 9}), 9)
}

func TestMax_SingleValue(t *testing.T) {
	assertClose(t, Max([]float64{42}), 42)
}

func TestMax_Empty(t *testing.T) {
	assertClose(t, Max(nil), 0)
}

func TestMax_NegativeNumbers(t *testing.T) {
	assertClose(t, Max([]float64{-5, -1, -10}), -1)
}

func TestMax_IdenticalValues(t *testing.T) {
	assertClose(t, Max([]float64{7, 7, 7}), 7)
}

func TestCompute_MultipleAggs(t *testing.T) {
	values := []float64{10, 20, 30, 40, 50}
	result := Compute(values, []string{"median", "mean", "min", "max"})

	assertClose(t, result["median"], 30)
	assertClose(t, result["mean"], 30)
	assertClose(t, result["min"], 10)
	assertClose(t, result["max"], 50)
}

func TestCompute_EmptyValues(t *testing.T) {
	result := Compute(nil, []string{"median", "mean", "p95", "min", "max"})
	for _, k := range []string{"median", "mean", "p95", "min", "max"} {
		if result[k] != 0 {
			t.Errorf("expected 0 for %s with empty input, got %v", k, result[k])
		}
	}
}

func TestCompute_UnknownAgg(t *testing.T) {
	result := Compute([]float64{1, 2, 3}, []string{"bogus"})
	if _, ok := result["bogus"]; ok {
		t.Error("unknown aggregation should not appear in results")
	}
}

func TestCompute_AllAggs(t *testing.T) {
	values := []float64{10, 20, 30, 40, 50}
	result := Compute(values, []string{"median", "mean", "p95", "min", "max"})
	if len(result) != 5 {
		t.Errorf("expected 5 results, got %d", len(result))
	}
}

func TestCompute_P95Only(t *testing.T) {
	result := Compute([]float64{1, 2, 3, 4, 5}, []string{"p95"})
	if _, ok := result["p95"]; !ok {
		t.Error("expected p95 in results")
	}
}
