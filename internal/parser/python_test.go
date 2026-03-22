package parser

import "testing"

func TestPythonParser_Detect(t *testing.T) {
	logs := readTestData(t, "pytest.txt")
	p := &PythonParser{}
	if !p.Detect(logs) {
		t.Error("expected Detect to return true for pytest output")
	}
}

func TestPythonParser_Detect_NoMatch(t *testing.T) {
	p := &PythonParser{}
	if p.Detect("just some random log output") {
		t.Error("expected Detect to return false for unrelated logs")
	}
}

func TestPythonParser_Extract(t *testing.T) {
	logs := readTestData(t, "pytest.txt")
	p := &PythonParser{}
	failures := p.Extract(logs)

	if len(failures) != 3 {
		t.Fatalf("expected 3 failures, got %d", len(failures))
	}

	// Check the first failure.
	found := false
	for _, f := range failures {
		if f.TestName == "test_login_with_expired_token" {
			found = true
			if f.Framework != "pytest" {
				t.Errorf("unexpected framework: %s", f.Framework)
			}
			if f.Location == "" {
				t.Error("expected non-empty location")
			}
			break
		}
	}
	if !found {
		t.Error("expected to find test_login_with_expired_token failure")
	}
}

func TestPythonParser_Extract_ClassMethod(t *testing.T) {
	logs := readTestData(t, "pytest.txt")
	p := &PythonParser{}
	failures := p.Extract(logs)

	found := false
	for _, f := range failures {
		if f.TestName == "test_retry_on_503" {
			found = true
			if f.Location == "" {
				t.Error("expected non-empty location for class method test")
			}
			break
		}
	}
	if !found {
		t.Error("expected to find test_retry_on_503 failure")
	}
}

func TestPythonParser_Extract_EmptyLogs(t *testing.T) {
	p := &PythonParser{}
	failures := p.Extract("")
	if len(failures) != 0 {
		t.Errorf("expected 0 failures for empty logs, got %d", len(failures))
	}
}

func TestPythonParser_Name(t *testing.T) {
	p := &PythonParser{}
	if p.Name() != "pytest" {
		t.Errorf("expected name 'pytest', got %q", p.Name())
	}
}

func TestPythonParser_NoFalsePositive(t *testing.T) {
	// Go test output should not trigger the Python parser.
	goLogs := readTestData(t, "go_test.txt")
	p := &PythonParser{}
	if p.Detect(goLogs) {
		t.Error("expected Detect to return false for go test output")
	}
}
