package parser

import "testing"

func TestGoParser_Detect(t *testing.T) {
	logs := readTestData(t, "go_test.txt")
	p := &GoParser{}
	if !p.Detect(logs) {
		t.Error("expected Detect to return true for go test output")
	}
}

func TestGoParser_Detect_NoMatch(t *testing.T) {
	p := &GoParser{}
	if p.Detect("just some random log output") {
		t.Error("expected Detect to return false for unrelated logs")
	}
}

func TestGoParser_Extract(t *testing.T) {
	logs := readTestData(t, "go_test.txt")
	p := &GoParser{}
	failures := p.Extract(logs)

	if len(failures) != 3 {
		t.Fatalf("expected 3 failures, got %d", len(failures))
	}

	// Check the first specific failure
	found := false
	for _, f := range failures {
		if f.TestName == "TestSearchService_Query/handles_empty_query" {
			found = true
			if f.Duration != "0.01s" {
				t.Errorf("unexpected duration: %s", f.Duration)
			}
			if f.Framework != "go test" {
				t.Errorf("unexpected framework: %s", f.Framework)
			}
			break
		}
	}
	if !found {
		t.Error("expected to find TestSearchService_Query/handles_empty_query failure")
	}
}

func TestGoParser_Extract_EmptyLogs(t *testing.T) {
	p := &GoParser{}
	failures := p.Extract("")
	if len(failures) != 0 {
		t.Errorf("expected 0 failures for empty logs, got %d", len(failures))
	}
}

func TestGoParser_Name(t *testing.T) {
	p := &GoParser{}
	if p.Name() != "go test" {
		t.Errorf("expected name 'go test', got %q", p.Name())
	}
}
