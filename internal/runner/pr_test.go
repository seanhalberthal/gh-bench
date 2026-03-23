package runner

import "testing"

func TestListOpenPRBranches(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["pr list"] = "feature-a\nfix-b\nfeature-c\n"

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	branches, err := ListOpenPRBranches()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(branches) != 3 {
		t.Fatalf("expected 3 branches, got %d", len(branches))
	}
	for _, name := range []string{"feature-a", "fix-b", "feature-c"} {
		if !branches[name] {
			t.Errorf("expected branch %q to be present", name)
		}
	}
}

func TestListOpenPRBranches_Empty(t *testing.T) {
	stub := newStubExecutor()
	stub.handlers["pr list"] = ""

	orig := Executor
	Executor = stub
	defer func() { Executor = orig }()

	branches, err := ListOpenPRBranches()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(branches) != 0 {
		t.Fatalf("expected 0 branches, got %d", len(branches))
	}
}
