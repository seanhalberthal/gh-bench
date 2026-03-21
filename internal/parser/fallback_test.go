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

func TestFallbackParser_Name(t *testing.T) {
	p := &FallbackParser{}
	if p.Name() != "unknown" {
		t.Errorf("expected name 'unknown', got %q", p.Name())
	}
}
