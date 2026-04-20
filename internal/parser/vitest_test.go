package parser

import (
	"strings"
	"testing"
)

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

func TestVitestParser_Detect_Typecheck(t *testing.T) {
	logs := readTestData(t, "vitest_typecheck.txt")
	p := &VitestParser{}
	if !p.Detect(logs) {
		t.Error("expected Detect to return true for vitest --typecheck output")
	}
}

func TestVitestParser_Extract_Typecheck(t *testing.T) {
	logs := readTestData(t, "vitest_typecheck.txt")
	p := &VitestParser{}
	failures := p.Extract(logs)

	if len(failures) != 2 {
		t.Fatalf("expected 2 failures, got %d", len(failures))
	}

	f1 := failures[0]
	if f1.TestName != "convex/beds.test.ts:7:29" {
		t.Errorf("unexpected test name: %q", f1.TestName)
	}
	if f1.Location != "convex/beds.test.ts:7:29" {
		t.Errorf("unexpected location: %q", f1.Location)
	}
	if f1.Framework != "Vitest" {
		t.Errorf("unexpected framework: %q", f1.Framework)
	}
	if !strings.Contains(f1.Message, "TS2339") {
		t.Errorf("expected message to contain TS code, got %q", f1.Message)
	}
	if !strings.Contains(f1.Message, "ImportMeta") {
		t.Errorf("expected message to contain tsc diagnostic text, got %q", f1.Message)
	}

	f2 := failures[1]
	if f2.TestName != "convex/memberships.test.ts:7:29" {
		t.Errorf("unexpected test name: %q", f2.TestName)
	}
}

func TestVitestParser_Extract_TypecheckSkippedWhenRuntimeFailuresPresent(t *testing.T) {
	// When the log contains both a runtime failure and a stray tsc-shaped
	// line, the runtime failure should win and the typecheck fallback
	// should not fire.
	logs := strings.Join([]string{
		" ✗ src/foo.test.ts (1 test) 10ms",
		"   ✗ Foo > bar",
		"      AssertionError: expected 1 to equal 2",
		"      at src/foo.test.ts:3:4",
		"convex/beds.test.ts:7:29 - error TS2339: Property 'glob' does not exist on type 'ImportMeta'.",
		"To ignore failing typecheck, use `--typecheck=disable`.",
	}, "\n")

	p := &VitestParser{}
	failures := p.Extract(logs)

	if len(failures) != 1 {
		t.Fatalf("expected 1 runtime failure (typecheck fallback should be skipped), got %d", len(failures))
	}
	if failures[0].TestName != "Foo > bar" {
		t.Errorf("expected runtime test name, got %q", failures[0].TestName)
	}
}
