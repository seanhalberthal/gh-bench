package parser

import "testing"

func TestVitestParser_Detect(t *testing.T) {
	logs := readTestData(t, "vitest.txt")
	p := &VitestParser{}
	if !p.Detect(logs) {
		t.Error("expected Detect to return true for Vitest output")
	}
}

func TestVitestParser_Detect_NoMatch(t *testing.T) {
	p := &VitestParser{}
	if p.Detect("just some random log output") {
		t.Error("expected Detect to return false for unrelated logs")
	}
}

func TestVitestParser_Extract(t *testing.T) {
	logs := readTestData(t, "vitest.txt")
	p := &VitestParser{}
	failures := p.Extract(logs)

	if len(failures) != 2 {
		t.Fatalf("expected 2 failures, got %d", len(failures))
	}

	f1 := failures[0]
	if f1.TestName != "HearingCard > renders participant names" {
		t.Errorf("unexpected test name: %q", f1.TestName)
	}
	if f1.Framework != "Vitest" {
		t.Errorf("unexpected framework: %s", f1.Framework)
	}
	if f1.Location == "" {
		t.Error("expected location to be extracted")
	}

	f2 := failures[1]
	if f2.TestName != "HearingCard > displays correct date format" {
		t.Errorf("unexpected test name: %q", f2.TestName)
	}
}

func TestVitestParser_Extract_EmptyLogs(t *testing.T) {
	p := &VitestParser{}
	failures := p.Extract("")
	if len(failures) != 0 {
		t.Errorf("expected 0 failures for empty logs, got %d", len(failures))
	}
}

func TestVitestParser_Name(t *testing.T) {
	p := &VitestParser{}
	if p.Name() != "Vitest" {
		t.Errorf("expected name 'Vitest', got %q", p.Name())
	}
}
