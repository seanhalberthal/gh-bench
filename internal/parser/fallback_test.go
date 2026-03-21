package parser

import (
	"strings"
	"testing"
)

func TestFallbackParser_Detect(t *testing.T) {
	p := &FallbackParser{}
	if p.Detect("anything") {
		t.Error("Detect should always return false")
	}
}

func TestFallbackParser_Extract(t *testing.T) {
	logs := readTestData(t, "unknown.txt")
	p := &FallbackParser{}
	failures := p.Extract(logs)

	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(failures))
	}

	f := failures[0]
	if f.TestName != "(unstructured output)" {
		t.Errorf("unexpected test name: %q", f.TestName)
	}
	if f.Framework != "unknown" {
		t.Errorf("unexpected framework: %s", f.Framework)
	}
	if f.Message == "" {
		t.Error("expected message to contain log tail")
	}
}

func TestFallbackParser_Extract_EmptyLogs(t *testing.T) {
	p := &FallbackParser{}
	failures := p.Extract("")
	if len(failures) != 0 {
		t.Errorf("expected 0 failures for empty logs, got %d", len(failures))
	}
}

func TestFallbackParser_Extract_ShortLogs(t *testing.T) {
	p := &FallbackParser{}
	failures := p.Extract("line one\nline two\nline three")
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(failures))
	}
	if !strings.Contains(failures[0].Message, "line one") {
		t.Error("short logs should include all lines")
	}
}

func TestFallbackParser_Extract_LongLogs(t *testing.T) {
	var lines []string
	for range 50 {
		lines = append(lines, "log line")
	}
	logs := strings.Join(lines, "\n")

	p := &FallbackParser{}
	failures := p.Extract(logs)

	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(failures))
	}
	// Should only contain the last 30 lines
	outputLines := strings.Split(failures[0].Message, "\n")
	if len(outputLines) != 30 {
		t.Errorf("expected 30 lines, got %d", len(outputLines))
	}
}

func TestFallbackParser_Extract_FiltersBoilerplate(t *testing.T) {
	logs := `some actual output
##[group]Run dotnet test
##[endgroup]
shell: /usr/bin/bash -e {0}
env:
  DOTNET_NOLOGO: true
  DOTNET_CLI_TELEMETRY_OPTOUT: true
  GCP_REGION: europe-west2
Backend tests failed
##[error]Process completed with exit code 1.`

	p := &FallbackParser{}
	failures := p.Extract(logs)

	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(failures))
	}

	msg := failures[0].Message
	// Should contain the actual error line
	if !strings.Contains(msg, "Backend tests failed") {
		t.Errorf("expected 'Backend tests failed' in output, got: %s", msg)
	}
	// Should NOT contain env var boilerplate
	if strings.Contains(msg, "DOTNET_NOLOGO") {
		t.Errorf("expected env vars to be filtered out, got: %s", msg)
	}
	if strings.Contains(msg, "GCP_REGION") {
		t.Errorf("expected env vars to be filtered out, got: %s", msg)
	}
}

func TestFallbackParser_Extract_ErrorPrioritised(t *testing.T) {
	logs := `lots of output
more output
##[error]Build failed: missing dependency foo`

	p := &FallbackParser{}
	failures := p.Extract(logs)

	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(failures))
	}

	// ##[error] message should be the primary signal
	if !strings.HasPrefix(failures[0].Message, "Build failed: missing dependency foo") {
		t.Errorf("expected ##[error] message first, got: %s", failures[0].Message)
	}
}

func TestFallbackParser_Extract_GenericExitCodeFiltered(t *testing.T) {
	logs := `Backend tests failed
##[error]Process completed with exit code 1.`

	p := &FallbackParser{}
	failures := p.Extract(logs)

	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(failures))
	}

	// The generic "Process completed with exit code 1." should not be the message.
	// The actual signal "Backend tests failed" should be shown instead.
	if strings.Contains(failures[0].Message, "Process completed with exit code") {
		t.Errorf("expected generic exit code message to be filtered, got: %s", failures[0].Message)
	}
	if !strings.Contains(failures[0].Message, "Backend tests failed") {
		t.Errorf("expected actual error in output, got: %s", failures[0].Message)
	}
}

func TestFallbackParser_Extract_ShellScriptFiltered(t *testing.T) {
	logs := `if [ "failure" != "success" ]; then
  echo "Backend tests failed"
  exit 1
fi
Backend tests failed`

	p := &FallbackParser{}
	failures := p.Extract(logs)

	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(failures))
	}

	msg := failures[0].Message
	if strings.Contains(msg, "if [") {
		t.Errorf("expected shell conditionals to be filtered, got: %s", msg)
	}
	if strings.Contains(msg, "fi") && !strings.Contains(msg, "failed") {
		t.Errorf("expected 'fi' to be filtered, got: %s", msg)
	}
	if !strings.Contains(msg, "Backend tests failed") {
		t.Errorf("expected actual error message, got: %s", msg)
	}
}

func TestFallbackParser_Extract_RealCIStatusCheck(t *testing.T) {
	logs := readTestData(t, "gh_actions_status_check.txt")
	p := &FallbackParser{}
	failures := p.Extract(logs)

	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(failures))
	}

	msg := failures[0].Message
	// Must contain the actual error signal
	if !strings.Contains(msg, "Backend tests failed") {
		t.Errorf("expected 'Backend tests failed' in output, got:\n%s", msg)
	}
	// Must NOT contain env var boilerplate
	if strings.Contains(msg, "DOTNET_NOLOGO") {
		t.Errorf("expected env vars to be filtered, got:\n%s", msg)
	}
	if strings.Contains(msg, "GCP_REGION") {
		t.Errorf("expected env vars to be filtered, got:\n%s", msg)
	}
	if strings.Contains(msg, "CI_ARTEFACTS_BUCKET") {
		t.Errorf("expected env vars to be filtered, got:\n%s", msg)
	}
	// Must NOT contain shell: line
	if strings.Contains(msg, "shell: /usr/bin/bash") {
		t.Errorf("expected shell line to be filtered, got:\n%s", msg)
	}
	// Must NOT contain ##[endgroup]
	if strings.Contains(msg, "##[endgroup]") {
		t.Errorf("expected ##[endgroup] to be filtered, got:\n%s", msg)
	}
}

func TestFallbackParser_Name(t *testing.T) {
	p := &FallbackParser{}
	if p.Name() != "unknown" {
		t.Errorf("expected name 'unknown', got %q", p.Name())
	}
}
