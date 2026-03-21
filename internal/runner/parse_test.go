package runner

import (
	"testing"
)

func TestExtractValues_ValidPattern(t *testing.T) {
	results := []RunResult{
		{RunID: 1, Title: "run one", Log: "build complete\nmedian=84.5ms\ndone"},
		{RunID: 2, Title: "run two", Log: "build complete\nmedian=91.0ms\ndone"},
		{RunID: 3, Title: "run three", Log: "build complete\nmedian=103.2ms\ndone"},
	}

	values, err := ExtractValues(results, `median=(?P<ms>[0-9.]+)ms`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 3 {
		t.Fatalf("expected 3 values, got %d", len(values))
	}

	expected := []float64{84.5, 91.0, 103.2}
	for i, v := range values {
		if v.Value != expected[i] {
			t.Errorf("values[%d] = %v, want %v", i, v.Value, expected[i])
		}
	}
}

func TestExtractValues_NoNamedGroup(t *testing.T) {
	_, err := ExtractValues(nil, `median=([0-9.]+)ms`)
	if err == nil {
		t.Fatal("expected error for pattern without named capture group")
	}
}

func TestExtractValues_InvalidRegex(t *testing.T) {
	_, err := ExtractValues(nil, `[invalid`)
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestExtractValues_NoMatch(t *testing.T) {
	results := []RunResult{
		{RunID: 1, Title: "run one", Log: "no match here"},
	}

	values, err := ExtractValues(results, `median=(?P<ms>[0-9.]+)ms`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 0 {
		t.Fatalf("expected 0 values, got %d", len(values))
	}
}

func TestExtractValues_NonNumericMatch(t *testing.T) {
	results := []RunResult{
		{RunID: 1, Title: "run one", Log: "status=abc"},
	}

	// Pattern that matches non-numeric text
	_, err := ExtractValues(results, `status=(?P<val>[a-z]+)`)
	if err == nil {
		t.Fatal("expected error for non-numeric match")
	}
}

func TestExtractValues_MultiLineLog(t *testing.T) {
	results := []RunResult{
		{RunID: 1, Title: "run", Log: "line1\nline2\nmetric=42.0\nline4\nmetric=99.0\nline6"},
	}

	values, err := ExtractValues(results, `metric=(?P<val>[0-9.]+)`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// First match per run
	if len(values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(values))
	}
	if values[0].Value != 42.0 {
		t.Errorf("expected first match (42.0), got %v", values[0].Value)
	}
}

func TestExtractValues_MultipleNamedGroups(t *testing.T) {
	results := []RunResult{
		{RunID: 1, Title: "run", Log: "time=100ms memory=512mb"},
	}

	// Pattern with multiple named groups — first named group is used
	values, err := ExtractValues(results, `time=(?P<time>[0-9]+)ms memory=(?P<mem>[0-9]+)mb`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(values))
	}
	if values[0].Value != 100.0 {
		t.Errorf("expected 100 (first named group), got %v", values[0].Value)
	}
}

func TestExtractedValues_Numbers(t *testing.T) {
	ev := ExtractedValues{
		{Value: 1.5},
		{Value: 2.5},
		{Value: 3.5},
	}
	nums := ev.Numbers()
	if len(nums) != 3 || nums[0] != 1.5 || nums[1] != 2.5 || nums[2] != 3.5 {
		t.Errorf("unexpected numbers: %v", nums)
	}
}

func TestExtractValues_EmptyResults(t *testing.T) {
	values, err := ExtractValues(nil, `metric=(?P<val>[0-9.]+)`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 0 {
		t.Fatalf("expected 0 values, got %d", len(values))
	}
}

func TestExtractValues_MixedMatchAndNoMatch(t *testing.T) {
	results := []RunResult{
		{RunID: 1, Title: "has match", Log: "metric=42.0"},
		{RunID: 2, Title: "no match", Log: "nothing here"},
		{RunID: 3, Title: "has match", Log: "metric=99.0"},
	}

	values, err := ExtractValues(results, `metric=(?P<val>[0-9.]+)`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(values))
	}
}
