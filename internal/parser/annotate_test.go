package parser

import "testing"

func TestExtractTimestamp(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		want  string
	}{
		{
			"standard timestamp",
			"2026-03-20T12:15:15.1234567Z --- FAIL: TestFoo (0.01s)",
			"20/03/26 12:15:15",
		},
		{
			"short fractional",
			"2026-01-05T09:30:00.1Z some output",
			"05/01/26 09:30:00",
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

	if failures[0].Timestamp != "20/03/26 12:15:10" {
		t.Errorf("failures[0].Timestamp = %q, want %q", failures[0].Timestamp, "20/03/26 12:15:10")
	}
	if failures[1].Timestamp != "20/03/26 12:15:11" {
		t.Errorf("failures[1].Timestamp = %q, want %q", failures[1].Timestamp, "20/03/26 12:15:11")
	}
}

func TestAnnotateTimestamps_Dotnet(t *testing.T) {
	rawLog := "2026-03-20T14:00:00.1234567Z   Failed Acme.Tests.SearchTests.TestQuery [45ms]\n" +
		"2026-03-20T14:00:00.2345678Z     System.NullReferenceException: Object reference\n"

	failures := []Failure{
		{TestName: "Acme.Tests.SearchTests.TestQuery", Framework: "dotnet"},
	}

	AnnotateTimestamps(failures, rawLog)

	if failures[0].Timestamp != "20/03/26 14:00:00" {
		t.Errorf("failures[0].Timestamp = %q, want %q", failures[0].Timestamp, "20/03/26 14:00:00")
	}
}

func TestAnnotateTimestamps_Vitest(t *testing.T) {
	rawLog := "2026-03-20T15:30:00.1234567Z  FAIL src/App.test.tsx\n" +
		"2026-03-20T15:30:01.1234567Z  ✗ App > renders correctly\n" +
		"2026-03-20T15:30:01.2345678Z    AssertionError: expected 1 to equal 2\n"

	failures := []Failure{
		{TestName: "App > renders correctly", Framework: "vitest"},
	}

	AnnotateTimestamps(failures, rawLog)

	if failures[0].Timestamp != "20/03/26 15:30:01" {
		t.Errorf("failures[0].Timestamp = %q, want %q", failures[0].Timestamp, "20/03/26 15:30:01")
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

	if failures[0].Timestamp != "20/03/26 16:00:00" {
		t.Errorf("failures[0].Timestamp = %q, want %q", failures[0].Timestamp, "20/03/26 16:00:00")
	}
	if failures[1].Timestamp != "20/03/26 16:00:01" {
		t.Errorf("failures[1].Timestamp = %q, want %q", failures[1].Timestamp, "20/03/26 16:00:01")
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
