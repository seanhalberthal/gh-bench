package parser

import "testing"

func TestParse_DotnetDetected(t *testing.T) {
	logs := readTestData(t, "dotnet_xunit.txt")
	failures := Parse(logs)
	if len(failures) == 0 {
		t.Fatal("expected failures to be extracted from dotnet logs")
	}
	if failures[0].Framework != "dotnet" {
		t.Errorf("expected framework 'dotnet', got %q", failures[0].Framework)
	}
}

func TestParse_GoDetected(t *testing.T) {
	logs := readTestData(t, "go_test.txt")
	failures := Parse(logs)
	if len(failures) == 0 {
		t.Fatal("expected failures to be extracted from go test logs")
	}
	if failures[0].Framework != "go test" {
		t.Errorf("expected framework 'go test', got %q", failures[0].Framework)
	}
}

func TestParse_VitestDetected(t *testing.T) {
	logs := readTestData(t, "vitest.txt")
	failures := Parse(logs)
	if len(failures) == 0 {
		t.Fatal("expected failures to be extracted from vitest logs")
	}
	if failures[0].Framework != "Vitest" {
		t.Errorf("expected framework 'Vitest', got %q", failures[0].Framework)
	}
}

func TestParse_PytestDetected(t *testing.T) {
	logs := readTestData(t, "pytest.txt")
	failures := Parse(logs)
	if len(failures) == 0 {
		t.Fatal("expected failures to be extracted from pytest logs")
	}
	if failures[0].Framework != "pytest" {
		t.Errorf("expected framework 'pytest', got %q", failures[0].Framework)
	}
}

func TestDetectFramework_Pytest(t *testing.T) {
	logs := readTestData(t, "pytest.txt")
	if fw := DetectFramework(logs); fw != "pytest" {
		t.Errorf("expected 'pytest', got %q", fw)
	}
}

func TestParse_FallbackForUnknown(t *testing.T) {
	logs := readTestData(t, "unknown.txt")
	failures := Parse(logs)
	if len(failures) == 0 {
		t.Fatal("expected fallback to produce a failure entry")
	}
	if failures[0].Framework != "unknown" {
		t.Errorf("expected framework 'unknown', got %q", failures[0].Framework)
	}
}

func TestParse_EmptyLogs(t *testing.T) {
	failures := Parse("")
	if len(failures) != 0 {
		t.Errorf("expected 0 failures for empty logs, got %d", len(failures))
	}
}

func TestDetectFramework_DotNet(t *testing.T) {
	logs := readTestData(t, "dotnet_xunit.txt")
	if fw := DetectFramework(logs); fw != "dotnet" {
		t.Errorf("expected 'dotnet', got %q", fw)
	}
}

func TestDetectFramework_Go(t *testing.T) {
	logs := readTestData(t, "go_test.txt")
	if fw := DetectFramework(logs); fw != "go test" {
		t.Errorf("expected 'go test', got %q", fw)
	}
}

func TestDetectFramework_Vitest(t *testing.T) {
	logs := readTestData(t, "vitest.txt")
	if fw := DetectFramework(logs); fw != "Vitest" {
		t.Errorf("expected 'Vitest', got %q", fw)
	}
}

func TestParse_VitestTypecheckDetected(t *testing.T) {
	logs := readTestData(t, "vitest_typecheck.txt")
	failures := Parse(logs)
	if len(failures) == 0 {
		t.Fatal("expected failures to be extracted from vitest typecheck logs")
	}
	if failures[0].Framework != "Vitest" {
		t.Errorf("expected framework 'Vitest', got %q", failures[0].Framework)
	}
}

func TestDetectFramework_VitestTypecheck(t *testing.T) {
	logs := readTestData(t, "vitest_typecheck.txt")
	if fw := DetectFramework(logs); fw != "Vitest" {
		t.Errorf("expected 'Vitest', got %q", fw)
	}
}

func TestDetectFramework_Unknown(t *testing.T) {
	if fw := DetectFramework("random text"); fw != "unknown" {
		t.Errorf("expected 'unknown', got %q", fw)
	}
}

func TestParse_FirstMatchWins(t *testing.T) {
	// Logs that contain both dotnet AND go test patterns
	// Dotnet is first in the parser list, so it should win
	mixed := `Failed SomeTest [10ms]
--- FAIL: TestOther (0.01s)
FAIL	some/package	0.5s`

	failures := Parse(mixed)
	if len(failures) == 0 {
		t.Fatal("expected at least one failure")
	}
	// Dotnet parser is first in the list and should detect first
	if failures[0].Framework != "dotnet" {
		t.Errorf("expected 'dotnet' (first match wins), got %q", failures[0].Framework)
	}
}
