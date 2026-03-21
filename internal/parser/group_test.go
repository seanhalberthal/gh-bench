package parser

import "testing"

func TestGroupFailures_SameTestMultipleRuns(t *testing.T) {
	runFailures := map[int64][]Failure{
		1: {{TestName: "TestFoo", Message: "expected true", Framework: "go test"}},
		2: {{TestName: "TestFoo", Message: "expected true", Framework: "go test"}},
		3: {{TestName: "TestFoo", Message: "expected true", Framework: "go test"}},
	}

	groups := GroupFailures(runFailures)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Count != 3 {
		t.Errorf("expected count=3, got %d", groups[0].Count)
	}
	if groups[0].TestName != "TestFoo" {
		t.Errorf("expected TestFoo, got %q", groups[0].TestName)
	}
}

func TestGroupFailures_DifferentTests(t *testing.T) {
	runFailures := map[int64][]Failure{
		1: {{TestName: "TestFoo", Message: "err", Framework: "go test"}},
		2: {{TestName: "TestBar", Message: "err", Framework: "go test"}},
	}

	groups := GroupFailures(runFailures)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
}

func TestGroupFailures_SameTestDifferentMessages(t *testing.T) {
	runFailures := map[int64][]Failure{
		1: {{TestName: "TestFoo", Message: "error A", Framework: "go test"}},
		2: {{TestName: "TestFoo", Message: "error B", Framework: "go test"}},
	}

	groups := GroupFailures(runFailures)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups (different messages), got %d", len(groups))
	}
}

func TestGroupFailures_Empty(t *testing.T) {
	groups := GroupFailures(map[int64][]Failure{})
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups, got %d", len(groups))
	}
}

func TestGroupFailures_SortedByCount(t *testing.T) {
	runFailures := map[int64][]Failure{
		1: {{TestName: "TestRare", Message: "err", Framework: "go test"}},
		2: {
			{TestName: "TestCommon", Message: "err", Framework: "go test"},
		},
		3: {
			{TestName: "TestCommon", Message: "err", Framework: "go test"},
		},
		4: {
			{TestName: "TestCommon", Message: "err", Framework: "go test"},
		},
	}

	groups := GroupFailures(runFailures)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0].TestName != "TestCommon" {
		t.Errorf("expected most common first, got %q", groups[0].TestName)
	}
	if groups[0].Count != 3 {
		t.Errorf("expected count=3, got %d", groups[0].Count)
	}
}

func TestGroupKey_Deterministic(t *testing.T) {
	f := Failure{TestName: "TestFoo", Message: "error\nstack trace", Framework: "go test"}
	key1 := GroupKey(f)
	key2 := GroupKey(f)
	if key1 != key2 {
		t.Errorf("GroupKey not deterministic: %q != %q", key1, key2)
	}
}

func TestGroupKey_FirstLineOnly(t *testing.T) {
	f1 := Failure{TestName: "TestFoo", Message: "error\nstack trace A", Framework: "go test"}
	f2 := Failure{TestName: "TestFoo", Message: "error\nstack trace B", Framework: "go test"}
	if GroupKey(f1) != GroupKey(f2) {
		t.Error("expected same key when first line matches (stack traces differ)")
	}
}

func TestGroupFailures_RunIDsSorted(t *testing.T) {
	runFailures := map[int64][]Failure{
		99: {{TestName: "TestFoo", Message: "err", Framework: "go test"}},
		1:  {{TestName: "TestFoo", Message: "err", Framework: "go test"}},
		50: {{TestName: "TestFoo", Message: "err", Framework: "go test"}},
	}

	groups := GroupFailures(runFailures)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	ids := groups[0].RunIDs
	for i := 1; i < len(ids); i++ {
		if ids[i] < ids[i-1] {
			t.Errorf("RunIDs not sorted: %v", ids)
			break
		}
	}
}
