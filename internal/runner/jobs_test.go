package runner

import (
	"testing"
)

func TestGetFailedSteps_SingleFailure(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run view 100 --json jobs"] = `{
		"jobs": [
			{
				"id": 1001,
				"name": "build",
				"status": "completed",
				"conclusion": "success",
				"steps": []
			},
			{
				"id": 1002,
				"name": "test",
				"status": "completed",
				"conclusion": "failure",
				"steps": [
					{"name": "Setup", "status": "completed", "conclusion": "success", "number": 1},
					{"name": "Run tests", "status": "completed", "conclusion": "failure", "number": 2}
				]
			}
		]
	}`
	stub.handlers["run view 100 --log --job 1002"] = "test\tRun tests\tFAIL: something went wrong\ntest\tRun tests\terror at line 42"

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	steps, err := GetFailedSteps(100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 failed step, got %d", len(steps))
	}
	if steps[0].Name != "Run tests" {
		t.Errorf("unexpected step name: %q", steps[0].Name)
	}
}

func TestGetFailedSteps_NoFailures(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run view 200 --json jobs"] = `{
		"jobs": [
			{
				"id": 2001,
				"name": "build",
				"status": "completed",
				"conclusion": "success",
				"steps": [
					{"name": "Build", "status": "completed", "conclusion": "success", "number": 1}
				]
			}
		]
	}`

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	steps, err := GetFailedSteps(200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 0 {
		t.Fatalf("expected 0 failed steps, got %d", len(steps))
	}
}

func TestGetFailedSteps_InvalidJSON(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run view 300 --json jobs"] = `not valid json`

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	_, err := GetFailedSteps(300)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestExtractStepLog_MatchesPrefix(t *testing.T) {
	fullLog := "test\tRun tests\tline 1\ntest\tRun tests\tline 2\ntest\tSetup\tsetup line"
	result := extractStepLog(fullLog, "test", "Run tests")
	if result != "line 1\nline 2" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestExtractStepLog_NoMatch(t *testing.T) {
	fullLog := "some random log output"
	result := extractStepLog(fullLog, "test", "Run tests")
	// No tabs, so stripAllPrefixes returns lines as-is
	if result != fullLog {
		t.Errorf("expected full log fallback, got: %q", result)
	}
}

func TestExtractStepLog_NoMatch_StripsPrefixes(t *testing.T) {
	// When step name doesn't match any discovered prefix, the fallback
	// should still strip job\tstep\t prefixes from all lines.
	fullLog := "job1\tUNKNOWN STEP\tline 1\njob1\tUNKNOWN STEP\tline 2"
	result := extractStepLog(fullLog, "job1", "Run tests")
	if result != "line 1\nline 2" {
		t.Errorf("expected prefixes stripped in fallback, got: %q", result)
	}
}

func TestExtractStepLog_JobNameMismatch(t *testing.T) {
	// Simulates API returning display name "Test Summary" while log uses YAML key "test-summary"
	fullLog := "test-summary\tRun tests\tline 1\ntest-summary\tRun tests\tline 2\ntest-summary\tSetup\tsetup line"
	result := extractStepLog(fullLog, "Test Summary", "Run tests")
	// Should match by step name even though job name differs
	if result != "line 1\nline 2" {
		t.Errorf("expected fuzzy match on step name, got: %q", result)
	}
}

func TestExtractStepLog_CaseInsensitive(t *testing.T) {
	fullLog := "ci\trun Tests\tline 1\nci\trun Tests\tline 2"
	result := extractStepLog(fullLog, "ci", "Run Tests")
	if result != "line 1\nline 2" {
		t.Errorf("expected case-insensitive match, got: %q", result)
	}
}

func TestDiscoverPrefixes(t *testing.T) {
	lines := []string{
		"job1\tstep1\tcontent",
		"job1\tstep1\tmore content",
		"job1\tstep2\tother content",
		"job2\tstep3\tdifferent job",
		"no tabs here",
	}
	prefixes := discoverPrefixes(lines)
	if len(prefixes) != 3 {
		t.Fatalf("expected 3 unique prefixes, got %d: %v", len(prefixes), prefixes)
	}
}

func TestShouldSkipStep(t *testing.T) {
	tests := []struct {
		name string
		skip bool
	}{
		{"Set up job", true},
		{"Complete job", true},
		{"Post Run actions/checkout@v4", true},
		{"Post Run actions/cache@v3", true},
		{"Initialize containers", true},
		{"Run tests", false},
		{"Run integration-platform tests", false},
		{"Build", false},
		{"Check CI status", false},
	}

	for _, tt := range tests {
		if got := shouldSkipStep(tt.name); got != tt.skip {
			t.Errorf("shouldSkipStep(%q) = %v, want %v", tt.name, got, tt.skip)
		}
	}
}

func TestGetFailedSteps_FallbackToFullLog(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run view 400 --json jobs"] = `{
		"jobs": [
			{
				"id": 4001,
				"name": "test",
				"status": "completed",
				"conclusion": "failure",
				"steps": [
					{"name": "Run tests", "status": "completed", "conclusion": "failure", "number": 1}
				]
			}
		]
	}`
	// Per-job log fails, full log succeeds
	stub.handlers["run view 400 --log"] = "full log output for entire run"

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	steps, err := GetFailedSteps(400)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 failed step, got %d", len(steps))
	}
}
