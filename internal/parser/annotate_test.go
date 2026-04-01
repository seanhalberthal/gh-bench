package parser

import (
	"fmt"
	"strings"
	"testing"
)

func TestExtractTimestamp(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{
			"standard timestamp (GMT)",
			"2026-03-20T12:15:15.1234567Z --- FAIL: TestFoo (0.01s)",
			"20/03/26 12:15:15 GMT",
		},
		{
			"short fractional (GMT)",
			"2026-01-05T09:30:00.1Z some output",
			"05/01/26 09:30:00 GMT",
		},
		{
			"BST timestamp",
			"2026-07-15T14:30:00.1234567Z some summer output",
			"15/07/26 15:30:00 BST",
		},
		{
			"no timestamp",
			"--- FAIL: TestFoo (0.01s)",
			"",
		},
		{
			"empty line",
			"",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTimestamp(tt.line)
			if got != tt.want {
				t.Errorf("extractTimestamp(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestAnnotateTimestamps_GoTest(t *testing.T) {
	rawLog := "2026-03-20T12:15:10.1234567Z === RUN   TestFoo\n" +
		"2026-03-20T12:15:10.2345678Z     foo_test.go:42: expected 1, got 2\n" +
		"2026-03-20T12:15:10.3456789Z --- FAIL: TestFoo (0.01s)\n" +
		"2026-03-20T12:15:11.1234567Z === RUN   TestBar\n" +
		"2026-03-20T12:15:11.2345678Z     bar_test.go:10: wrong result\n" +
		"2026-03-20T12:15:11.3456789Z --- FAIL: TestBar (0.02s)\n"

	failures := []Failure{
		{TestName: "TestFoo", Framework: "go test"},
		{TestName: "TestBar", Framework: "go test"},
	}

	AnnotateTimestamps(failures, rawLog)

	if failures[0].Timestamp != "20/03/26 12:15:10 GMT" {
		t.Errorf("failures[0].Timestamp = %q, want %q", failures[0].Timestamp, "20/03/26 12:15:10 GMT")
	}
	if failures[1].Timestamp != "20/03/26 12:15:11 GMT" {
		t.Errorf("failures[1].Timestamp = %q, want %q", failures[1].Timestamp, "20/03/26 12:15:11 GMT")
	}
}

func TestAnnotateTimestamps_Dotnet(t *testing.T) {
	rawLog := "2026-03-20T14:00:00.1234567Z   Failed Acme.Tests.SearchTests.TestQuery [45ms]\n" +
		"2026-03-20T14:00:00.2345678Z     System.NullReferenceException: Object reference\n"

	failures := []Failure{
		{TestName: "Acme.Tests.SearchTests.TestQuery", Framework: "dotnet"},
	}

	AnnotateTimestamps(failures, rawLog)

	if failures[0].Timestamp != "20/03/26 14:00:00 GMT" {
		t.Errorf("failures[0].Timestamp = %q, want %q", failures[0].Timestamp, "20/03/26 14:00:00 GMT")
	}
}

func TestAnnotateTimestamps_Vitest(t *testing.T) {
	rawLog := "2026-03-20T15:30:00.1234567Z  FAIL src/App.test.tsx\n" +
		"2026-03-20T15:30:01.1234567Z  ✗ App > renders correctly\n" +
		"2026-03-20T15:30:01.2345678Z    AssertionError: expected 1 to equal 2\n"

	failures := []Failure{
		{TestName: "App > renders correctly", Framework: "Vitest"},
	}

	AnnotateTimestamps(failures, rawLog)

	if failures[0].Timestamp != "20/03/26 15:30:01 GMT" {
		t.Errorf("failures[0].Timestamp = %q, want %q", failures[0].Timestamp, "20/03/26 15:30:01 GMT")
	}
}

func TestAnnotateTimestamps_Pytest(t *testing.T) {
	rawLog := "2026-03-20T16:00:00.1234567Z FAILED tests/test_auth.py::test_login - AssertionError\n" +
		"2026-03-20T16:00:01.1234567Z FAILED tests/test_auth.py::test_signup - ValueError\n"

	failures := []Failure{
		{TestName: "test_login", Framework: "pytest"},
		{TestName: "test_signup", Framework: "pytest"},
	}

	AnnotateTimestamps(failures, rawLog)

	if failures[0].Timestamp != "20/03/26 16:00:00 GMT" {
		t.Errorf("failures[0].Timestamp = %q, want %q", failures[0].Timestamp, "20/03/26 16:00:00 GMT")
	}
	if failures[1].Timestamp != "20/03/26 16:00:01 GMT" {
		t.Errorf("failures[1].Timestamp = %q, want %q", failures[1].Timestamp, "20/03/26 16:00:01 GMT")
	}
}

func TestAnnotateTimestamps_Unknown(t *testing.T) {
	rawLog := "2026-03-20T16:00:00.1234567Z some error output\n"

	failures := []Failure{
		{TestName: "(unstructured output)", Framework: "unknown"},
	}

	AnnotateTimestamps(failures, rawLog)

	if failures[0].Timestamp != "" {
		t.Errorf("expected empty timestamp for unknown framework, got %q", failures[0].Timestamp)
	}
}

func TestAnnotateTimestamps_EmptyInputs(t *testing.T) {
	// Empty failures — should not panic.
	AnnotateTimestamps(nil, "2026-03-20T16:00:00.1234567Z content")

	// Empty rawLog — should not panic.
	failures := []Failure{{TestName: "TestFoo", Framework: "go test"}}
	AnnotateTimestamps(failures, "")
	if failures[0].Timestamp != "" {
		t.Errorf("expected empty timestamp for empty rawLog, got %q", failures[0].Timestamp)
	}
}

func TestAnnotateTimestamps_NoTimestampInRawLog(t *testing.T) {
	rawLog := "--- FAIL: TestFoo (0.01s)\n"

	failures := []Failure{
		{TestName: "TestFoo", Framework: "go test"},
	}

	AnnotateTimestamps(failures, rawLog)

	if failures[0].Timestamp != "" {
		t.Errorf("expected empty timestamp when raw log has no timestamps, got %q", failures[0].Timestamp)
	}
}

func TestAnnotateTimestamps_Vitest_RoundTrip(t *testing.T) {
	// Simulate the full pipeline: Parse() produces failures with Framework: "Vitest",
	// then AnnotateTimestamps matches them against a raw log with timestamps.
	cleanLog := readTestData(t, "vitest.txt")
	failures := Parse(cleanLog)
	if len(failures) == 0 {
		t.Fatal("expected at least one failure from vitest testdata")
	}
	if failures[0].Framework != "Vitest" {
		t.Fatalf("expected framework %q, got %q", "Vitest", failures[0].Framework)
	}

	// Construct a raw log by prefixing each line with a timestamp.
	var rawLog string
	for i, line := range splitLines(cleanLog) {
		ts := "2026-03-20T15:30:" + fmt.Sprintf("%02d", i) + ".1234567Z "
		rawLog += ts + line + "\n"
	}

	AnnotateTimestamps(failures, rawLog)

	for i, f := range failures {
		if f.Timestamp == "" {
			t.Errorf("failures[%d] (%q) has empty timestamp after round-trip annotation", i, f.TestName)
		}
	}
}

func TestExtractTimestamp_BST(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{
			"summer date converts UTC to BST (+1)",
			"2026-07-15T14:30:00.1234567Z test output",
			"15/07/26 15:30:00 BST",
		},
		{
			"winter date stays GMT (+0)",
			"2026-12-01T14:30:00.1234567Z test output",
			"01/12/26 14:30:00 GMT",
		},
		{
			"just before spring forward (still GMT)",
			// 2026: clocks go forward last Sunday of March = 29 March at 01:00 UTC
			"2026-03-29T00:59:59.1234567Z test output",
			"29/03/26 00:59:59 GMT",
		},
		{
			"just after spring forward (now BST)",
			"2026-03-29T01:00:01.1234567Z test output",
			"29/03/26 02:00:01 BST",
		},
		{
			"just before autumn fallback (still BST)",
			// 2026: clocks go back last Sunday of October = 25 October at 01:00 UTC
			"2026-10-25T00:59:59.1234567Z test output",
			"25/10/26 01:59:59 BST",
		},
		{
			"just after autumn fallback (now GMT)",
			"2026-10-25T01:00:01.1234567Z test output",
			"25/10/26 01:00:01 GMT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTimestamp(tt.line)
			if got != tt.want {
				t.Errorf("extractTimestamp(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestAnnotateTimestamps_BST_Conversion(t *testing.T) {
	// Verify that summer timestamps are converted from UTC to BST in annotation.
	rawLog := "2026-07-15T14:00:00.1234567Z --- FAIL: TestSummer (0.01s)\n"

	failures := []Failure{
		{TestName: "TestSummer", Framework: "go test"},
	}

	AnnotateTimestamps(failures, rawLog)

	// 14:00 UTC = 15:00 BST
	want := "15/07/26 15:00:00 BST"
	if failures[0].Timestamp != want {
		t.Errorf("failures[0].Timestamp = %q, want %q", failures[0].Timestamp, want)
	}
}

func splitLines(s string) []string {
	return strings.Split(s, "\n")
}
