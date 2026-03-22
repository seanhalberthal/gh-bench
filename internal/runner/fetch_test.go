package runner

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// stubExecutor records calls and returns canned responses.
type stubExecutor struct {
	calls    [][]string
	handlers map[string]string
	err      error
}

func newStubExecutor() *stubExecutor {
	return &stubExecutor{
		handlers: make(map[string]string),
	}
}

func (s *stubExecutor) Run(args ...string) (string, error) {
	s.calls = append(s.calls, args)
	if s.err != nil {
		return "", s.err
	}
	key := strings.Join(args, " ")
	for pattern, response := range s.handlers {
		if strings.Contains(key, pattern) {
			return response, nil
		}
	}
	return "", fmt.Errorf("no handler for: %s", key)
}

func TestFetchLogs_WithRunIDs(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run view 100 --json displayTitle"] = "run alpha\t2025-03-20T10:00:00Z"
	stub.handlers["run view 100 --log"] = "log output for run 100"
	stub.handlers["run view 200 --json displayTitle"] = "run beta\t2025-03-19T09:00:00Z"
	stub.handlers["run view 200 --log"] = "log output for run 200"

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	results, err := FetchLogs(context.Background(), FetchOpts{
		RunIDs:      []int64{100, 200},
		Concurrency: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestFetchLogs_EmptyRunIDs(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run list"] = ""

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	results, err := FetchLogs(context.Background(), FetchOpts{
		Workflow:    "test.yml",
		Concurrency: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestFetchLogs_DefaultConcurrency(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run view 1 --json displayTitle"] = "test\t2025-01-01"
	stub.handlers["run view 1 --log"] = "logs"

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	results, err := FetchLogs(context.Background(), FetchOpts{
		RunIDs: []int64{1},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestListRunIDs_WithWorkflowAndBranch(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run list"] = "111\n222\n333\n"

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	ids, err := listRunIDs(FetchOpts{
		Workflow: "ci.yml",
		Branch:   "main",
		Limit:    5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 IDs, got %d", len(ids))
	}

	// Verify the right flags were passed
	if len(stub.calls) == 0 {
		t.Fatal("expected at least one call")
	}
	call := strings.Join(stub.calls[0], " ")
	if !strings.Contains(call, "--workflow ci.yml") {
		t.Error("expected --workflow flag")
	}
	if !strings.Contains(call, "--branch main") {
		t.Error("expected --branch flag")
	}
}

func TestListRunIDs_FailedOnly(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run list"] = "111\n"

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	_, err := listRunIDs(FetchOpts{
		Workflow:   "ci.yml",
		Limit:      5,
		FailedOnly: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	call := strings.Join(stub.calls[0], " ")
	if !strings.Contains(call, "--status failure") {
		t.Error("expected --status failure flag for FailedOnly")
	}
}

func TestListRunIDs_EmptyOutput(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run list"] = ""

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	ids, err := listRunIDs(FetchOpts{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected 0 IDs, got %d", len(ids))
	}
}

func TestFilterExcludedSteps(t *testing.T) {
	steps := []StepResult{
		{Name: "Run tests"},
		{Name: "Check CI status"},
		{Name: "Build and deploy"},
		{Name: "Verify CI Status Check"},
	}

	tests := []struct {
		name     string
		excludes []string
		want     []string
	}{
		{"no excludes", nil, []string{"Run tests", "Check CI status", "Build and deploy", "Verify CI Status Check"}},
		{"exclude one", []string{"CI status"}, []string{"Run tests", "Build and deploy"}},
		{"case insensitive", []string{"check ci"}, []string{"Run tests", "Build and deploy", "Verify CI Status Check"}},
		{"multiple excludes", []string{"CI status", "deploy"}, []string{"Run tests"}},
		{"no match", []string{"lint"}, []string{"Run tests", "Check CI status", "Build and deploy", "Verify CI Status Check"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Copy input to avoid mutation across subtests.
			input := make([]StepResult, len(steps))
			copy(input, steps)

			got := filterExcludedSteps(input, tt.excludes)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d steps, want %d", len(got), len(tt.want))
			}
			for i, s := range got {
				if s.Name != tt.want[i] {
					t.Errorf("step[%d] = %q, want %q", i, s.Name, tt.want[i])
				}
			}
		})
	}
}

func TestListRunIDs_InvalidID(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run list"] = "abc\n"

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	_, err := listRunIDs(FetchOpts{Limit: 10})
	if err == nil {
		t.Fatal("expected error for invalid ID")
	}
}
