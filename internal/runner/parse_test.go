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

func TestResolvePattern_ValidPreset(t *testing.T) {
	pattern, err := ResolvePattern("duration")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pattern != Presets["duration"].Pattern {
		t.Errorf("got %q, want %q", pattern, Presets["duration"].Pattern)
	}
}

func TestResolvePattern_UnknownPreset(t *testing.T) {
	_, err := ResolvePattern("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown preset")
	}
}

func TestPresetNames_Sorted(t *testing.T) {
	names := PresetNames()
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Fatalf("names not sorted: %v", names)
		}
	}
}

func TestAllPresets_ValidRegex(t *testing.T) {
	for name, preset := range Presets {
		t.Run(name, func(t *testing.T) {
			results := []RunResult{
				{RunID: 1, Title: "test", Log: preset.Example},
			}
			values, err := ExtractValues(results, preset.Pattern)
			if err != nil {
				t.Fatalf("preset %q failed on its own example: %v", name, err)
			}
			if len(values) == 0 {
				t.Errorf("preset %q matched nothing on its own example %q", name, preset.Example)
			}
		})
	}
}

func TestPreset_Duration(t *testing.T) {
	cases := []struct {
		log  string
		want float64
	}{
		{"Took 12.5s end", 12.5},
		{"duration: 45ms done", 45},
		{"elapsed= 3.2s", 3.2},
		{"Finished in 100ms", 100},
		{"completed in 7.8s", 7.8},
		{"Time: 4.589 s", 4.589},
	}
	pattern := Presets["duration"].Pattern
	for _, tc := range cases {
		results := []RunResult{{RunID: 1, Title: "t", Log: tc.log}}
		values, err := ExtractValues(results, pattern)
		if err != nil {
			t.Errorf("log %q: unexpected error: %v", tc.log, err)
			continue
		}
		if len(values) != 1 {
			t.Errorf("log %q: expected 1 value, got %d", tc.log, len(values))
			continue
		}
		if values[0].Value != tc.want {
			t.Errorf("log %q: got %v, want %v", tc.log, values[0].Value, tc.want)
		}
	}
}

func TestPreset_Coverage(t *testing.T) {
	cases := []struct {
		log  string
		want float64
	}{
		{"Coverage: 85.2%", 85.2},
		{"coverage=91%", 91},
		{"total coverage: 100.0 %", 100.0},
	}
	pattern := Presets["coverage"].Pattern
	for _, tc := range cases {
		results := []RunResult{{RunID: 1, Title: "t", Log: tc.log}}
		values, err := ExtractValues(results, pattern)
		if err != nil {
			t.Errorf("log %q: unexpected error: %v", tc.log, err)
			continue
		}
		if len(values) != 1 {
			t.Errorf("log %q: expected 1 value, got %d", tc.log, len(values))
			continue
		}
		if values[0].Value != tc.want {
			t.Errorf("log %q: got %v, want %v", tc.log, values[0].Value, tc.want)
		}
	}
}

func TestPreset_GoTest(t *testing.T) {
	results := []RunResult{
		{RunID: 1, Title: "t", Log: "ok  \tgithub.com/foo/bar\t1.234s"},
	}
	values, err := ExtractValues(results, Presets["go-test"].Pattern)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 1 || values[0].Value != 1.234 {
		t.Errorf("got %v, want 1.234", values)
	}
}

func TestPreset_Pytest(t *testing.T) {
	results := []RunResult{
		{RunID: 1, Title: "t", Log: "====== 42 passed in 3.45s ======"},
	}
	values, err := ExtractValues(results, Presets["pytest"].Pattern)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 1 || values[0].Value != 3.45 {
		t.Errorf("got %v, want 3.45", values)
	}
}
