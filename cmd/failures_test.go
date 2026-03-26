package cmd

import (
	"slices"
	"testing"

	"github.com/seanhalberthal/gh-bench/internal/runner"
)

func TestFilterByOpenPRs(t *testing.T) {
	tests := []struct {
		name     string
		runs     string // JSON response for run list
		prs      string // newline-separated branch names
		wantIDs  []int64
		wantNone bool
	}{
		{
			"matches open PR branches",
			`[{"databaseId":1,"headBranch":"feat-a"},{"databaseId":2,"headBranch":"main"},{"databaseId":3,"headBranch":"feat-b"}]`,
			"feat-a\nfeat-b",
			[]int64{1, 3},
			false,
		},
		{
			"no matching branches",
			`[{"databaseId":1,"headBranch":"main"}]`,
			"feat-a",
			nil,
			true,
		},
		{
			"empty runs",
			`[]`,
			"feat-a",
			nil,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &stubGHExecutor{handlers: map[string]string{
				"run list": tt.runs,
				"pr list":  tt.prs,
			}}
			orig := runner.Executor
			runner.Executor = stub
			defer func() { runner.Executor = orig }()

			ids, err := filterByOpenPRs(runner.FetchOpts{
				Workflow: "test.yml",
				Limit:    10,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNone {
				if len(ids) != 0 {
					t.Errorf("expected no IDs, got %v", ids)
				}
				return
			}
			slices.Sort(ids)
			if !slices.Equal(ids, tt.wantIDs) {
				t.Errorf("got %v, want %v", ids, tt.wantIDs)
			}
		})
	}
}

// stubGHExecutor is a minimal stub for tests that need to override runner.Executor.
type stubGHExecutor struct {
	handlers map[string]string
}

func (s *stubGHExecutor) Run(args ...string) (string, error) {
	key := args[0]
	for pattern, response := range s.handlers {
		if key == pattern || len(args) > 1 && args[0]+" "+args[1] == pattern {
			return response, nil
		}
	}
	return "", nil
}
