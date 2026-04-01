package runner

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// stubExecutor records calls and returns canned responses.
type stubExecutor struct {
	mu       sync.Mutex
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
	s.mu.Lock()
	s.calls = append(s.calls, args)
	s.mu.Unlock()
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
	stub.handlers["run view 100 --json displayTitle"] = "run alpha\t2025-03-20T10:00:00Z\tmain"
	stub.handlers["run view 100 --log"] = "log output for run 100"
	stub.handlers["run view 200 --json displayTitle"] = "run beta\t2025-03-19T09:00:00Z\tfeat-x"
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

	// Verify field-level parsing for first result.
	if results[0].RunID != 100 {
		t.Errorf("results[0].RunID = %d, want 100", results[0].RunID)
	}
	if results[0].Title != "run alpha" {
		t.Errorf("results[0].Title = %q, want %q", results[0].Title, "run alpha")
	}
	if results[0].Date != "2025-03-20T10:00:00Z" {
		t.Errorf("results[0].Date = %q, want %q", results[0].Date, "2025-03-20T10:00:00Z")
	}
	if results[0].Branch != "main" {
		t.Errorf("results[0].Branch = %q, want %q", results[0].Branch, "main")
	}
	if !strings.Contains(results[0].Log, "log output for run 100") {
		t.Errorf("results[0].Log = %q, want it to contain log output", results[0].Log)
	}

	// Verify second result.
	if results[1].RunID != 200 {
		t.Errorf("results[1].RunID = %d, want 200", results[1].RunID)
	}
	if results[1].Branch != "feat-x" {
		t.Errorf("results[1].Branch = %q, want %q", results[1].Branch, "feat-x")
	}
}

func TestFetchLogs_EmptyRunIDs(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run list"] = "[]"

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
	stub.handlers["run view 1 --json displayTitle"] = "test\t2025-01-01T00:00:00Z\tmain"
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
	stub.handlers["run list"] = `[{"databaseId":111,"headBranch":"feat-a"},{"databaseId":222,"headBranch":"feat-b"},{"databaseId":333,"headBranch":"main"}]`

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
	stub.handlers["run list"] = `[{"databaseId":111,"headBranch":"main"}]`

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
	stub.handlers["run list"] = "[]"

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

func TestListRuns_ReturnsBranchInfo(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run list"] = `[{"databaseId":100,"headBranch":"feature-x"},{"databaseId":200,"headBranch":"fix-y"}]`

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	runs, err := ListRuns(FetchOpts{Workflow: "ci.yml", Limit: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}
	if runs[0].ID != 100 || runs[0].Branch != "feature-x" {
		t.Errorf("run[0] = {%d, %q}, want {100, \"feature-x\"}", runs[0].ID, runs[0].Branch)
	}
	if runs[1].ID != 200 || runs[1].Branch != "fix-y" {
		t.Errorf("run[1] = {%d, %q}, want {200, \"fix-y\"}", runs[1].ID, runs[1].Branch)
	}
}

func TestListRuns_Empty(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run list"] = "[]"

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	runs, err := ListRuns(FetchOpts{Limit: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("expected 0 runs, got %d", len(runs))
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

func TestListRunIDs_InvalidJSON(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run list"] = "not json"

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	_, err := listRunIDs(FetchOpts{Limit: 10})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestFetchSingleRun_FailedOnly(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run view 42 --json displayTitle"] = "my run\t2025-01-01T10:00:00Z\tmain"
	stub.handlers["run view 42 --json jobs"] = `{"jobs":[{"databaseId":99,"name":"test","status":"completed","conclusion":"failure","steps":[{"name":"Run tests","status":"completed","conclusion":"failure","number":1}]}]}`
	stub.handlers["api repos/{owner}/{repo}/actions/jobs/99/logs"] = "FAIL\nerror details"

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	result, err := fetchSingleRun(context.Background(), 42, fetchRunOpts{FailedOnly: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RunID != 42 {
		t.Errorf("RunID = %d, want 42", result.RunID)
	}
	if result.Title != "my run" {
		t.Errorf("Title = %q, want %q", result.Title, "my run")
	}
	if len(result.FailedSteps) != 1 {
		t.Fatalf("expected 1 failed step, got %d", len(result.FailedSteps))
	}
	if result.FailedSteps[0].Name != "Run tests" {
		t.Errorf("step name = %q, want %q", result.FailedSteps[0].Name, "Run tests")
	}
	if !strings.Contains(result.FailedSteps[0].Log, "FAIL") {
		t.Errorf("step log should contain FAIL, got %q", result.FailedSteps[0].Log)
	}
}

func TestFetchSingleRun_StepFilter(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run view 50 --json displayTitle"] = "build run\t2025-02-01T12:00:00Z\tfeat"
	stub.handlers["run view 50 --json jobs"] = `{"jobs":[{"databaseId":77,"name":"build","status":"completed","conclusion":"success","steps":[{"name":"Run Tests","status":"completed","conclusion":"success","number":1}]}]}`
	stub.handlers["api repos/{owner}/{repo}/actions/jobs/77/logs"] = "2026-01-01T00:00:00.1Z test output line"

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	result, err := fetchSingleRun(context.Background(), 50, fetchRunOpts{Step: "run tests"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Log, "test output line") {
		t.Errorf("expected step log content, got %q", result.Log)
	}
}

func TestFetchSingleRun_MetadataError(t *testing.T) {
	stub := newStubExecutor()
	stub.err = fmt.Errorf("network error")

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	_, err := fetchSingleRun(context.Background(), 1, fetchRunOpts{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "fetching metadata") {
		t.Errorf("error = %q, want it to mention fetching metadata", err.Error())
	}
}

func TestGetStepLog_Found(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run view 10 --json jobs"] = `{"jobs":[{"databaseId":5,"name":"build","status":"completed","conclusion":"success","steps":[{"name":"Run Tests","status":"completed","conclusion":"success","number":1}]}]}`
	stub.handlers["api repos/{owner}/{repo}/actions/jobs/5/logs"] = "2026-01-01T00:00:00.1Z test output"

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	log, err := getStepLog(context.Background(), 10, "run tests")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(log, "test output") {
		t.Errorf("expected log to contain 'test output', got %q", log)
	}
}

func TestGetStepLog_NotFound(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run view 10 --json jobs"] = `{"jobs":[{"databaseId":5,"name":"build","status":"completed","conclusion":"success","steps":[{"name":"Deploy","status":"completed","conclusion":"success","number":1}]}]}`

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	_, err := getStepLog(context.Background(), 10, "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing step")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to mention 'not found'", err.Error())
	}
}

func TestGetStepLog_CaseInsensitive(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["run view 10 --json jobs"] = `{"jobs":[{"databaseId":5,"name":"build","status":"completed","conclusion":"success","steps":[{"name":"Run Tests","status":"completed","conclusion":"success","number":1}]}]}`
	stub.handlers["api repos/{owner}/{repo}/actions/jobs/5/logs"] = "2026-01-01T00:00:00.1Z matched"

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	log, err := getStepLog(context.Background(), 10, "RUN TESTS")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(log, "matched") {
		t.Errorf("expected case-insensitive match, got %q", log)
	}
}
