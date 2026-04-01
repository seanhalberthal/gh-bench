package parser

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// localTS converts a UTC RFC 3339 timestamp to the expected local display format.
func localTS(t *testing.T, utc string) string {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339Nano, utc)
	if err != nil {
		t.Fatalf("localTS: invalid timestamp %q: %v", utc, err)
	}
	return parsed.In(time.Local).Format(timestampFormat)
}

func TestExtractTimestamp(t *testing.T) {
	tests := []struct {
		name string
		line string
		utc  string // empty means expect ""
	}{
		{
			"standard timestamp",
			"2026-03-20T12:15:15.1234567Z --- FAIL: TestFoo (0.01s)",
			"2026-03-20T12:15:15.1234567Z",
		},
		{
			"short fractional",
			"2026-01-05T09:30:00.1Z some output",
			"2026-01-05T09:30:00.1Z",
		},
		{
			"summer timestamp",
			"2026-07-15T14:30:00.1234567Z some summer output",
			"2026-07-15T14:30:00.1234567Z",
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
			want := ""
			if tt.utc != "" {
				want = localTS(t, tt.utc)
			}
			if got != want {
				t.Errorf("extractTimestamp(%q) = %q, want %q", tt.line, got, want)
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

	wantFoo := localTS(t, "2026-03-20T12:15:10.3456789Z")
	if failures[0].Timestamp != wantFoo {
		t.Errorf("failures[0].Timestamp = %q, want %q", failures[0].Timestamp, wantFoo)
	}
	wantBar := localTS(t, "2026-03-20T12:15:11.3456789Z")
	if failures[1].Timestamp != wantBar {
		t.Errorf("failures[1].Timestamp = %q, want %q", failures[1].Timestamp, wantBar)
	}
}

func TestAnnotateTimestamps_Dotnet(t *testing.T) {
	rawLog := "2026-03-20T14:00:00.1234567Z   Failed Acme.Tests.SearchTests.TestQuery [45ms]\n" +
		"2026-03-20T14:00:00.2345678Z     System.NullReferenceException: Object reference\n"

	failures := []Failure{
		{TestName: "Acme.Tests.SearchTests.TestQuery", Framework: "dotnet"},
	}

	AnnotateTimestamps(failures, rawLog)

	want := localTS(t, "2026-03-20T14:00:00.1234567Z")
	if failures[0].Timestamp != want {
		t.Errorf("failures[0].Timestamp = %q, want %q", failures[0].Timestamp, want)
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

	want := localTS(t, "2026-03-20T15:30:01.1234567Z")
	if failures[0].Timestamp != want {
		t.Errorf("failures[0].Timestamp = %q, want %q", failures[0].Timestamp, want)
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

	wantLogin := localTS(t, "2026-03-20T16:00:00.1234567Z")
	if failures[0].Timestamp != wantLogin {
		t.Errorf("failures[0].Timestamp = %q, want %q", failures[0].Timestamp, wantLogin)
	}
	wantSignup := localTS(t, "2026-03-20T16:00:01.1234567Z")
	if failures[1].Timestamp != wantSignup {
		t.Errorf("failures[1].Timestamp = %q, want %q", failures[1].Timestamp, wantSignup)
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

func TestExtractTimestamp_LocalConversion(t *testing.T) {
	tests := []struct {
		name string
		line string
		utc  string
	}{
		{
			"summer date",
			"2026-07-15T14:30:00.1234567Z test output",
			"2026-07-15T14:30:00.1234567Z",
		},
		{
			"winter date",
			"2026-12-01T14:30:00.1234567Z test output",
			"2026-12-01T14:30:00.1234567Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTimestamp(tt.line)
			want := localTS(t, tt.utc)
			if got != want {
				t.Errorf("extractTimestamp(%q) = %q, want %q", tt.line, got, want)
			}
		})
	}
}

func TestAnnotateTimestamps_LocalConversion(t *testing.T) {
	rawLog := "2026-07-15T14:00:00.1234567Z --- FAIL: TestSummer (0.01s)\n"

	failures := []Failure{
		{TestName: "TestSummer", Framework: "go test"},
	}

	AnnotateTimestamps(failures, rawLog)

	want := localTS(t, "2026-07-15T14:00:00.1234567Z")
	if failures[0].Timestamp != want {
		t.Errorf("failures[0].Timestamp = %q, want %q", failures[0].Timestamp, want)
	}
}

func splitLines(s string) []string {
	return strings.Split(s, "\n")
}
