package runner

import (
	"context"
	"strings"
	"testing"
)

func TestGetFailedSteps_SingleFailure(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run view 100 --json jobs"] = `{
		"jobs": [
			{
				"databaseId": 1001,
				"name": "build",
				"status": "completed",
				"conclusion": "success",
				"steps": []
			},
			{
				"databaseId": 1002,
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
	// API-style raw log (no tab prefixes).
	stub.handlers["api repos/{owner}/{repo}/actions/jobs/1002/logs"] = "FAIL: something went wrong\nerror at line 42"

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	steps, err := GetFailedSteps(context.Background(), 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 failed step, got %d", len(steps))
	}
	if steps[0].Name != "Run tests" {
		t.Errorf("unexpected step name: %q", steps[0].Name)
	}
	if !strings.Contains(steps[0].Log, "FAIL: something went wrong") {
		t.Errorf("expected log to contain failure output, got: %q", steps[0].Log)
	}
}

func TestGetFailedSteps_FallsBackToRunView(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run view 100 --json jobs"] = `{
		"jobs": [
			{
				"databaseId": 1002,
				"name": "test",
				"status": "completed",
				"conclusion": "failure",
				"steps": [
					{"name": "Run tests", "status": "completed", "conclusion": "failure", "number": 1}
				]
			}
		]
	}`
	// API fails, fallback to gh run view --log --job
	stub.handlers["run view 100 --log --job 1002"] = "test\tRun tests\tline 1\ntest\tRun tests\tline 2"

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	steps, err := GetFailedSteps(context.Background(), 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 failed step, got %d", len(steps))
	}
	// Fallback should strip tab prefixes.
	if strings.Contains(steps[0].Log, "test\t") {
		t.Errorf("expected tab prefixes to be stripped, got: %q", steps[0].Log)
	}
	if !strings.Contains(steps[0].Log, "line 1") {
		t.Errorf("expected content to be preserved, got: %q", steps[0].Log)
	}
}

func TestGetFailedSteps_NoFailures(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run view 200 --json jobs"] = `{
		"jobs": [
			{
				"databaseId": 2001,
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

	steps, err := GetFailedSteps(context.Background(), 200)
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

	_, err := GetFailedSteps(context.Background(), 300)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestGetFailedSteps_SkipsInfrastructureSteps(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run view 500 --json jobs"] = `{
		"jobs": [
			{
				"databaseId": 5001,
				"name": "test",
				"status": "completed",
				"conclusion": "failure",
				"steps": [
					{"name": "Set up job", "status": "completed", "conclusion": "failure", "number": 1},
					{"name": "Run tests", "status": "completed", "conclusion": "failure", "number": 2},
					{"name": "Post Run actions/checkout@v4", "status": "completed", "conclusion": "failure", "number": 3},
					{"name": "Complete job", "status": "completed", "conclusion": "failure", "number": 4}
				]
			}
		]
	}`
	stub.handlers["api repos/{owner}/{repo}/actions/jobs/5001/logs"] = "test output"

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	steps, err := GetFailedSteps(context.Background(), 500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only "Run tests" should remain — the others are infrastructure.
	if len(steps) != 1 {
		t.Fatalf("expected 1 step (infrastructure filtered), got %d", len(steps))
	}
	if steps[0].Name != "Run tests" {
		t.Errorf("unexpected step: %q", steps[0].Name)
	}
}

func TestStripLogPrefixes(t *testing.T) {
	input := "job1\tStep1\tline 1\njob1\tStep1\tline 2\nno tabs here"
	result := stripLogPrefixes(input)
	if result != "line 1\nline 2\nno tabs here" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestStripLogPrefixes_WithTimestamps(t *testing.T) {
	// Real-world GitHub Actions log format: job\tstep\ttimestamp content
	input := "test\tRun tests\t2026-03-16T13:34:37.3465175Z ok  \tgithub.com/foo/bar\t1.234s\n" +
		"test\tRun tests\t2026-03-16T13:34:37.3558618Z ok  \tgithub.com/foo/baz\t0.567s\n" +
		"no tabs here"
	got := stripLogPrefixes(input)
	want := "ok  \tgithub.com/foo/bar\t1.234s\nok  \tgithub.com/foo/baz\t0.567s\nno tabs here"
	if got != want {
		t.Errorf("stripLogPrefixes() = %q, want %q", got, want)
	}
}

func TestStripTabPrefixesOnly(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"strips tab prefixes preserves timestamps",
			"test\tRun tests\t2026-03-16T13:34:37.3465175Z content here\n" +
				"test\tRun tests\t2026-03-16T13:34:38.1234567Z more content",
			"2026-03-16T13:34:37.3465175Z content here\n" +
				"2026-03-16T13:34:38.1234567Z more content",
		},
		{
			"no tabs preserves line",
			"no tabs here",
			"no tabs here",
		},
		{
			"single tab preserves line",
			"only\tone tab",
			"only\tone tab",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripTabPrefixesOnly(tt.input)
			if got != tt.want {
				t.Errorf("stripTabPrefixesOnly() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetFailedSteps_RawLogPreservesTimestamps(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run view 100 --json jobs"] = `{
		"jobs": [
			{
				"databaseId": 1002,
				"name": "test",
				"status": "completed",
				"conclusion": "failure",
				"steps": [
					{"name": "Run tests", "status": "completed", "conclusion": "failure", "number": 1}
				]
			}
		]
	}`
	// API-style raw log with timestamps.
	stub.handlers["api repos/{owner}/{repo}/actions/jobs/1002/logs"] = "2026-03-20T12:15:15.1234567Z --- FAIL: TestFoo (0.01s)\n2026-03-20T12:15:16.1234567Z FAIL"

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	steps, err := GetFailedSteps(context.Background(), 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	// Log should have timestamps stripped.
	if strings.Contains(steps[0].Log, "2026-03-20") {
		t.Errorf("Log should not contain timestamps, got: %q", steps[0].Log)
	}
	// RawLog should preserve timestamps.
	if !strings.Contains(steps[0].RawLog, "2026-03-20T12:15:15.1234567Z") {
		t.Errorf("RawLog should preserve timestamps, got: %q", steps[0].RawLog)
	}
}

func TestGetFailedSteps_FallbackRawLogPreservesTimestamps(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run view 100 --json jobs"] = `{
		"jobs": [
			{
				"databaseId": 1002,
				"name": "test",
				"status": "completed",
				"conclusion": "failure",
				"steps": [
					{"name": "Run tests", "status": "completed", "conclusion": "failure", "number": 1}
				]
			}
		]
	}`
	// API fails, fallback to gh run view --log --job (with tab prefixes + timestamps).
	stub.handlers["run view 100 --log --job 1002"] = "test\tRun tests\t2026-03-20T12:15:15.1234567Z --- FAIL: TestFoo"

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	steps, err := GetFailedSteps(context.Background(), 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	// Log should have both tab prefixes and timestamps stripped.
	if strings.Contains(steps[0].Log, "2026-03-20") || strings.Contains(steps[0].Log, "test\t") {
		t.Errorf("Log should be clean, got: %q", steps[0].Log)
	}
	// RawLog should have tab prefixes stripped but timestamps preserved.
	if !strings.Contains(steps[0].RawLog, "2026-03-20T12:15:15.1234567Z") {
		t.Errorf("RawLog should preserve timestamps, got: %q", steps[0].RawLog)
	}
	if strings.Contains(steps[0].RawLog, "test\t") {
		t.Errorf("RawLog should not contain tab prefixes, got: %q", steps[0].RawLog)
	}
}

func TestSegmentByStep(t *testing.T) {
	log := "##[group]Run tests\nFAIL: TestFoo\nerror at line 42\n##[endgroup]\n##[group]Generate coverage\ncoverage: 80%\n##[endgroup]"

	t.Run("extracts matching step", func(t *testing.T) {
		got := segmentByStep(log, "Run tests")
		want := "FAIL: TestFoo\nerror at line 42"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("extracts second step", func(t *testing.T) {
		got := segmentByStep(log, "Generate coverage")
		want := "coverage: 80%"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("case insensitive match", func(t *testing.T) {
		got := segmentByStep(log, "run tests")
		if !strings.Contains(got, "FAIL: TestFoo") {
			t.Errorf("expected case-insensitive match, got %q", got)
		}
	})

	t.Run("no markers falls back to full log", func(t *testing.T) {
		plain := "just some log output\nno markers here"
		got := segmentByStep(plain, "Run tests")
		if got != plain {
			t.Errorf("expected full log fallback, got %q", got)
		}
	})

	t.Run("no matching step falls back to full log", func(t *testing.T) {
		got := segmentByStep(log, "Nonexistent step")
		if got != log {
			t.Errorf("expected full log fallback, got %q", got)
		}
	})
}

func TestGetFailedSteps_MultipleStepsSegmented(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run view 100 --json jobs"] = `{
		"jobs": [
			{
				"databaseId": 1002,
				"name": "test",
				"status": "completed",
				"conclusion": "failure",
				"steps": [
					{"name": "Run tests", "status": "completed", "conclusion": "failure", "number": 1},
					{"name": "Generate coverage", "status": "completed", "conclusion": "failure", "number": 2}
				]
			}
		]
	}`
	stub.handlers["api repos/{owner}/{repo}/actions/jobs/1002/logs"] =
		"2026-03-20T12:15:15.1234567Z ##[group]Run tests\n" +
			"2026-03-20T12:15:16.1234567Z FAIL: TestFoo\n" +
			"2026-03-20T12:15:17.1234567Z ##[endgroup]\n" +
			"2026-03-20T12:15:18.1234567Z ##[group]Generate coverage\n" +
			"2026-03-20T12:15:19.1234567Z coverage: 80%\n" +
			"2026-03-20T12:15:20.1234567Z ##[endgroup]"

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	steps, err := GetFailedSteps(context.Background(), 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if !strings.Contains(steps[0].Log, "FAIL: TestFoo") {
		t.Errorf("step 0 should contain test failure, got %q", steps[0].Log)
	}
	if strings.Contains(steps[0].Log, "coverage: 80%") {
		t.Errorf("step 0 should not contain coverage output, got %q", steps[0].Log)
	}
	if !strings.Contains(steps[1].Log, "coverage: 80%") {
		t.Errorf("step 1 should contain coverage output, got %q", steps[1].Log)
	}
	if strings.Contains(steps[1].Log, "FAIL: TestFoo") {
		t.Errorf("step 1 should not contain test failure, got %q", steps[1].Log)
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
