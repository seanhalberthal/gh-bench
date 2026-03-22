package runner

import (
	"regexp"
	"testing"
)

// mustCompile is a test helper that compiles a pattern and fails the test on error.
func mustCompile(t *testing.T, pattern string) (*regexp.Regexp, int, int) {
	t.Helper()
	re, idx, labelIdx, err := CompilePattern(pattern)
	if err != nil {
		t.Fatalf("CompilePattern(%q): %v", pattern, err)
	}
	return re, idx, labelIdx
}

func TestExtractValues_ValidPattern(t *testing.T) {
	results := []RunResult{
		{RunID: 1, Title: "run one", Log: "build complete\nmedian=84.5ms\ndone"},
		{RunID: 2, Title: "run two", Log: "build complete\nmedian=91.0ms\ndone"},
		{RunID: 3, Title: "run three", Log: "build complete\nmedian=103.2ms\ndone"},
	}

	re, idx, labelIdx := mustCompile(t, `median=(?P<ms>[0-9.]+)ms`)
	values, err := ExtractValues(results, re, idx, labelIdx, false)
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

func TestCompilePattern_NoNamedGroup(t *testing.T) {
	_, _, _, err := CompilePattern(`median=([0-9.]+)ms`)
	if err == nil {
		t.Fatal("expected error for pattern without named capture group")
	}
}

func TestCompilePattern_InvalidRegex(t *testing.T) {
	_, _, _, err := CompilePattern(`[invalid`)
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestCompilePattern_OnlyLabelGroup(t *testing.T) {
	_, _, _, err := CompilePattern(`(?P<label>\S+)`)
	if err == nil {
		t.Fatal("expected error when only group is label (no value group)")
	}
}

func TestCompilePattern_WithLabelGroup(t *testing.T) {
	re, valIdx, labelIdx, err := CompilePattern(`^ok\s+(?P<label>\S+)\s+(?P<duration>[0-9.]+)s`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if labelIdx < 0 {
		t.Fatal("expected labelIdx >= 0")
	}

	match := re.FindStringSubmatch("ok  \tgithub.com/foo/bar\t1.234s")
	if match == nil {
		t.Fatal("expected match")
	}
	if match[valIdx] != "1.234" {
		t.Errorf("value group = %q, want %q", match[valIdx], "1.234")
	}
	if match[labelIdx] != "github.com/foo/bar" {
		t.Errorf("label group = %q, want %q", match[labelIdx], "github.com/foo/bar")
	}
}

func TestExtractValues_NoMatch(t *testing.T) {
	results := []RunResult{
		{RunID: 1, Title: "run one", Log: "no match here"},
	}

	re, idx, labelIdx := mustCompile(t, `median=(?P<ms>[0-9.]+)ms`)
	values, err := ExtractValues(results, re, idx, labelIdx, false)
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
	re, idx, labelIdx := mustCompile(t, `status=(?P<val>[a-z]+)`)
	_, err := ExtractValues(results, re, idx, labelIdx, false)
	if err == nil {
		t.Fatal("expected error for non-numeric match")
	}
}

func TestExtractValues_MultiLineLog(t *testing.T) {
	results := []RunResult{
		{RunID: 1, Title: "run", Log: "line1\nline2\nmetric=42.0\nline4\nmetric=99.0\nline6"},
	}

	re, idx, labelIdx := mustCompile(t, `metric=(?P<val>[0-9.]+)`)
	values, err := ExtractValues(results, re, idx, labelIdx, false)
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

	// Pattern with multiple named groups — first non-label named group is used
	re, idx, labelIdx := mustCompile(t, `time=(?P<time>[0-9]+)ms memory=(?P<mem>[0-9]+)mb`)
	values, err := ExtractValues(results, re, idx, labelIdx, false)
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

func TestExtractedValues_HasLabels(t *testing.T) {
	without := ExtractedValues{{Value: 1.0}, {Value: 2.0}}
	if without.HasLabels() {
		t.Error("expected HasLabels() = false for values without labels")
	}

	with := ExtractedValues{{Value: 1.0, Label: "pkg/foo"}, {Value: 2.0}}
	if !with.HasLabels() {
		t.Error("expected HasLabels() = true when at least one value has a label")
	}
}

func TestExtractValues_EmptyResults(t *testing.T) {
	re, idx, labelIdx := mustCompile(t, `metric=(?P<val>[0-9.]+)`)
	values, err := ExtractValues(nil, re, idx, labelIdx, false)
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

	re, idx, labelIdx := mustCompile(t, `metric=(?P<val>[0-9.]+)`)
	values, err := ExtractValues(results, re, idx, labelIdx, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(values))
	}
}

func TestExtractValues_WithLabels(t *testing.T) {
	results := []RunResult{
		{RunID: 1, Title: "commit msg", Log: "ok  \tgithub.com/foo/bar\t1.234s\nok  \tgithub.com/foo/baz\t0.567s"},
	}

	re, idx, labelIdx := mustCompile(t, `^ok\s+(?P<label>\S+)\s+(?P<duration>[0-9.]+)s`)
	values, err := ExtractValues(results, re, idx, labelIdx, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(values))
	}
	if values[0].Label != "github.com/foo/bar" {
		t.Errorf("values[0].Label = %q, want %q", values[0].Label, "github.com/foo/bar")
	}
	if values[0].Value != 1.234 {
		t.Errorf("values[0].Value = %v, want 1.234", values[0].Value)
	}
	if values[1].Label != "github.com/foo/baz" {
		t.Errorf("values[1].Label = %q, want %q", values[1].Label, "github.com/foo/baz")
	}
	if values[1].Value != 0.567 {
		t.Errorf("values[1].Value = %v, want 0.567", values[1].Value)
	}
}

func TestExtractValues_WithoutLabels(t *testing.T) {
	results := []RunResult{
		{RunID: 1, Title: "commit msg", Log: "metric=42.0"},
	}

	re, idx, labelIdx := mustCompile(t, `metric=(?P<val>[0-9.]+)`)
	values, err := ExtractValues(results, re, idx, labelIdx, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(values))
	}
	if values[0].Label != "" {
		t.Errorf("expected empty label, got %q", values[0].Label)
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
			re, idx, labelIdx := mustCompile(t, preset.Pattern)
			values, err := ExtractValues(results, re, idx, labelIdx, false)
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
	re, idx, labelIdx := mustCompile(t, Presets["duration"].Pattern)
	for _, tc := range cases {
		results := []RunResult{{RunID: 1, Title: "t", Log: tc.log}}
		values, err := ExtractValues(results, re, idx, labelIdx, false)
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
	re, idx, labelIdx := mustCompile(t, Presets["coverage"].Pattern)
	for _, tc := range cases {
		results := []RunResult{{RunID: 1, Title: "t", Log: tc.log}}
		values, err := ExtractValues(results, re, idx, labelIdx, false)
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
	re, idx, labelIdx := mustCompile(t, Presets["go-test"].Pattern)
	values, err := ExtractValues(results, re, idx, labelIdx, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 1 || values[0].Value != 1.234 {
		t.Errorf("got %v, want 1.234", values)
	}
	if values[0].Label != "github.com/foo/bar" {
		t.Errorf("label = %q, want %q", values[0].Label, "github.com/foo/bar")
	}
}

func TestPreset_Pytest(t *testing.T) {
	results := []RunResult{
		{RunID: 1, Title: "t", Log: "====== 42 passed in 3.45s ======"},
	}
	re, idx, labelIdx := mustCompile(t, Presets["pytest"].Pattern)
	values, err := ExtractValues(results, re, idx, labelIdx, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 1 || values[0].Value != 3.45 {
		t.Errorf("got %v, want 3.45", values)
	}
}

func TestExtractValues_MatchAll(t *testing.T) {
	results := []RunResult{
		{RunID: 1, Title: "run", Log: "line1\nmetric=42.0\nline3\nmetric=99.0\nline5"},
	}

	re, idx, labelIdx := mustCompile(t, `metric=(?P<val>[0-9.]+)`)
	values, err := ExtractValues(results, re, idx, labelIdx, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(values))
	}
	if values[0].Value != 42.0 {
		t.Errorf("values[0] = %v, want 42.0", values[0].Value)
	}
	if values[1].Value != 99.0 {
		t.Errorf("values[1] = %v, want 99.0", values[1].Value)
	}
}

func TestExtractValues_MatchAll_NoMatches(t *testing.T) {
	results := []RunResult{
		{RunID: 1, Title: "run", Log: "no matches here"},
	}

	re, idx, labelIdx := mustCompile(t, `metric=(?P<val>[0-9.]+)`)
	values, err := ExtractValues(results, re, idx, labelIdx, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 0 {
		t.Fatalf("expected 0 values, got %d", len(values))
	}
}

func TestExtractValues_MatchAll_MixedRuns(t *testing.T) {
	results := []RunResult{
		{RunID: 1, Title: "multi", Log: "metric=10.0\nmetric=20.0"},
		{RunID: 2, Title: "none", Log: "nothing"},
		{RunID: 3, Title: "single", Log: "metric=30.0"},
	}

	re, idx, labelIdx := mustCompile(t, `metric=(?P<val>[0-9.]+)`)
	values, err := ExtractValues(results, re, idx, labelIdx, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 3 {
		t.Fatalf("expected 3 values, got %d", len(values))
	}
}

// TestPresets_RealWorldLogs verifies that all presets correctly match real
// GitHub Actions log output after timestamp and prefix stripping.
// These samples are modelled on actual CI log lines.
func TestPresets_RealWorldLogs(t *testing.T) {
	tests := []struct {
		preset   string
		log      string // log content as it appears after stripLogPrefixes
		matchAll bool
		want     []float64
		labels   []string // expected labels (nil = don't check)
	}{
		// go-test: real supplyscan CI output (after prefix/timestamp stripping)
		{
			preset: "go-test",
			log: "=== RUN   TestAudit\n--- PASS: TestAudit (0.02s)\n" +
				"ok  \tgithub.com/seanhalberthal/supplyscan/internal/audit\t1.047s\n" +
				"ok  \tgithub.com/seanhalberthal/supplyscan/internal/cli\t1.049s\n" +
				"ok  \tgithub.com/seanhalberthal/supplyscan/internal/lockfile\t1.027s\n" +
				"ok  \tgithub.com/seanhalberthal/supplyscan/internal/scanner\t166.754s",
			matchAll: true,
			want:     []float64{1.047, 1.049, 1.027, 166.754},
			labels: []string{
				"github.com/seanhalberthal/supplyscan/internal/audit",
				"github.com/seanhalberthal/supplyscan/internal/cli",
				"github.com/seanhalberthal/supplyscan/internal/lockfile",
				"github.com/seanhalberthal/supplyscan/internal/scanner",
			},
		},
		// go-test: single package
		{
			preset:   "go-test",
			log:      "ok  \tgithub.com/foo/bar\t0.003s",
			matchAll: false,
			want:     []float64{0.003},
			labels:   []string{"github.com/foo/bar"},
		},
		// duration: "Took Xs" format
		{
			preset:   "duration",
			log:      "Build complete\nTook 12.5s\nUploading artefacts",
			matchAll: false,
			want:     []float64{12.5},
		},
		// duration: "duration: Xms" format
		{
			preset:   "duration",
			log:      "Running benchmark...\nduration: 245ms\nAll done",
			matchAll: false,
			want:     []float64{245},
		},
		// duration: "elapsed: Xs" format
		{
			preset:   "duration",
			log:      "Task finished\nelapsed: 3.2s\nExit code 0",
			matchAll: false,
			want:     []float64{3.2},
		},
		// duration: "Finished in Xms" format
		{
			preset:   "duration",
			log:      "Finished in 100ms",
			matchAll: false,
			want:     []float64{100},
		},
		// coverage: Go test coverage output
		{
			preset:   "coverage",
			log:      "ok  \tgithub.com/foo/bar\t1.234s\tcoverage: 85.2% of statements",
			matchAll: false,
			want:     []float64{85.2},
		},
		// coverage: Istanbul/nyc format
		{
			preset:   "coverage",
			log:      "All files   |   91.3 |    88.1 |     85 |   91.3\ncoverage: 91.3% total",
			matchAll: false,
			want:     []float64{91.3},
		},
		// jest: Jest time output
		{
			preset:   "jest",
			log:      "Test Suites: 12 passed, 12 total\nTests:       48 passed, 48 total\nTime:        4.589 s",
			matchAll: false,
			want:     []float64{4.589},
		},
		// jest: Vitest time output
		{
			preset:   "jest",
			log:      " Test Files  3 passed (3)\n Tests  15 passed (15)\n Duration  892 ms",
			matchAll: false,
			want:     []float64{892},
		},
		// pytest: standard pytest summary
		{
			preset:   "pytest",
			log:      "collected 42 items\n...\n====== 42 passed in 3.45s ======",
			matchAll: false,
			want:     []float64{3.45},
		},
		// pytest: with warnings
		{
			preset:   "pytest",
			log:      "====== 100 passed, 2 warnings in 12.67s ======",
			matchAll: false,
			want:     []float64{12.67},
		},
		// bundle-size: webpack output
		{
			preset:   "bundle-size",
			log:      "asset main.js 245.3 kB [emitted] [minimized]\nBundle size: 245.3 kB",
			matchAll: false,
			want:     []float64{245.3},
		},
		// bundle-size: custom output
		{
			preset:   "bundle-size",
			log:      "Build complete\nsize: 1.2 MB\nGzip: 312 kB",
			matchAll: false,
			want:     []float64{1.2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.preset+"_"+tt.log[:min(30, len(tt.log))], func(t *testing.T) {
			re, idx, labelIdx := mustCompile(t, Presets[tt.preset].Pattern)
			results := []RunResult{{RunID: 1, Title: "test", Log: tt.log}}
			values, err := ExtractValues(results, re, idx, labelIdx, tt.matchAll)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(values) != len(tt.want) {
				t.Fatalf("expected %d values, got %d", len(tt.want), len(values))
			}
			for i, v := range values {
				if v.Value != tt.want[i] {
					t.Errorf("values[%d] = %v, want %v", i, v.Value, tt.want[i])
				}
				if tt.labels != nil && v.Label != tt.labels[i] {
					t.Errorf("values[%d].Label = %q, want %q", i, v.Label, tt.labels[i])
				}
			}
		})
	}
}

// TestPresets_WithRawGitHubActionsLogs verifies that presets work end-to-end
// with raw GitHub Actions log lines (including job/step prefixes and timestamps)
// after being passed through stripLogPrefixes.
func TestPresets_WithRawGitHubActionsLogs(t *testing.T) {
	tests := []struct {
		preset string
		rawLog string // raw gh run view --log output
		want   []float64
	}{
		{
			preset: "go-test",
			rawLog: "test\tUNKNOWN STEP\t2026-03-16T13:34:37.3465175Z ok  \tgithub.com/seanhalberthal/supplyscan/internal/audit\t1.047s\n" +
				"test\tUNKNOWN STEP\t2026-03-16T13:34:37.3558618Z ok  \tgithub.com/seanhalberthal/supplyscan/internal/cli\t1.049s\n" +
				"test\tUNKNOWN STEP\t2026-03-16T13:37:24.1037868Z ok  \tgithub.com/seanhalberthal/supplyscan/internal/scanner\t166.754s",
			want: []float64{1.047, 1.049, 166.754},
		},
		{
			preset: "duration",
			rawLog: "build\tBuild\t2026-01-15T14:22:33.1234567Z Took 12.5s",
			want:   []float64{12.5},
		},
		{
			preset: "coverage",
			rawLog: "test\tRun tests\t2026-01-15T14:22:33.1234567Z coverage: 85.2% of statements",
			want:   []float64{85.2},
		},
		{
			preset: "jest",
			rawLog: "test\tRun tests\t2026-01-15T14:22:33.1234567Z Time:        4.589 s, estimated 5 s",
			want:   []float64{4.589},
		},
		{
			preset: "pytest",
			rawLog: "test\tRun tests\t2026-01-15T14:22:33.1234567Z ====== 42 passed in 3.45s ======",
			want:   []float64{3.45},
		},
		{
			preset: "bundle-size",
			rawLog: "build\tBuild\t2026-01-15T14:22:33.1234567Z Bundle size: 245.3 kB",
			want:   []float64{245.3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.preset, func(t *testing.T) {
			// Simulate the full pipeline: raw log → stripLogPrefixes → ExtractValues
			cleanedLog := stripLogPrefixes(tt.rawLog)
			re, idx, labelIdx := mustCompile(t, Presets[tt.preset].Pattern)
			results := []RunResult{{RunID: 1, Title: "test", Log: cleanedLog}}
			values, err := ExtractValues(results, re, idx, labelIdx, true)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(values) != len(tt.want) {
				t.Fatalf("expected %d values, got %d", len(tt.want), len(values))
			}
			for i, v := range values {
				if v.Value != tt.want[i] {
					t.Errorf("values[%d] = %v, want %v", i, v.Value, tt.want[i])
				}
			}
		})
	}
}
