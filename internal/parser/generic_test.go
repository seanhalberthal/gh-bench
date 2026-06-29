package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenericParser_ESLintStylish(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "eslint_stylish.txt"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	logs := string(data)

	p := &GenericParser{}
	if !p.Detect(logs) {
		t.Fatal("expected eslint stylish output to be detected")
	}

	failures := p.Extract(logs)
	if len(failures) != 2 {
		t.Fatalf("expected 2 findings, got %d: %+v", len(failures), failures)
	}

	err0 := failures[0]
	if err0.Framework != "eslint" {
		t.Errorf("framework = %q, want eslint", err0.Framework)
	}
	if err0.TestName != "@typescript-eslint/no-unnecessary-type-assertion" {
		t.Errorf("test name = %q", err0.TestName)
	}
	wantLoc := "/home/runner/work/tattoo-studio/tattoo-studio/apps/mobile/app/_layout.tsx:23:54"
	if err0.Location != wantLoc {
		t.Errorf("location = %q, want %q", err0.Location, wantLoc)
	}
	if err0.Message != "This assertion is unnecessary since the receiver accepts the original type of the expression" {
		t.Errorf("message = %q", err0.Message)
	}

	warn := failures[1]
	if warn.Message != "warning: 'foo' is assigned a value but never used" {
		t.Errorf("warning message = %q", warn.Message)
	}
}

func TestGenericParser_LineOriented(t *testing.T) {
	tests := []struct {
		name      string
		log       string
		framework string
		testName  string
		location  string
		message   string
	}{
		{
			"tsc parenthesised",
			"src/app.ts(12,5): error TS2322: Type 'string' is not assignable to type 'number'.",
			"tsc", "TS2322", "src/app.ts:12:5", "Type 'string' is not assignable to type 'number'.",
		},
		{
			"tsc pretty colon",
			"src/app.ts:12:5 - error TS2552: Cannot find name 'foo'.",
			"tsc", "TS2552", "src/app.ts:12:5", "Cannot find name 'foo'.",
		},
		{
			"golangci-lint",
			"internal/foo/bar.go:12:5: undefined: Foo (typecheck)",
			"golangci-lint", "typecheck", "internal/foo/bar.go:12:5", "undefined: Foo",
		},
		{
			"ruff",
			"app/main.py:1:8: F401 [*] 'os' imported but unused",
			"ruff", "F401", "app/main.py:1:8", "[*] 'os' imported but unused",
		},
		{
			"gcc no rule",
			"src/main.c:5:3: error: 'x' undeclared (first use in this function)",
			"gcc/clang", "src/main.c:5:3", "", "'x' undeclared (first use in this function)",
		},
	}

	p := &GenericParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// pad with a second diagnostic so single-line shapes also pass Detect
			logs := tt.log + "\n" + tt.log
			if !p.Detect(logs) {
				t.Fatalf("expected detection for %q", tt.log)
			}
			f := p.Extract(tt.log)
			if len(f) != 1 {
				t.Fatalf("expected 1 finding, got %d: %+v", len(f), f)
			}
			got := f[0]
			if got.Framework != tt.framework {
				t.Errorf("framework = %q, want %q", got.Framework, tt.framework)
			}
			if got.TestName != tt.testName {
				t.Errorf("test name = %q, want %q", got.TestName, tt.testName)
			}
			if got.Location != tt.location {
				t.Errorf("location = %q, want %q", got.Location, tt.location)
			}
			if got.Message != tt.message {
				t.Errorf("message = %q, want %q", got.Message, tt.message)
			}
		})
	}
}

func TestGenericParser_DetectNegative(t *testing.T) {
	p := &GenericParser{}
	negatives := []string{
		"just some unstructured build output\nnothing to see here",
		"make: *** [Makefile:371: lint] Error 1",       // not a diagnostic
		"see config at internal/foo/bar.go:12 for more", // single stray reference
	}
	for _, n := range negatives {
		if p.Detect(n) {
			t.Errorf("unexpected detection for %q", n)
		}
	}
}

func TestParse_RoutesESLintToGeneric(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "eslint_stylish.txt"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	failures := Parse(string(data))
	if len(failures) == 0 {
		t.Fatal("expected findings, got none")
	}
	if failures[0].Framework != "eslint" {
		t.Errorf("framework = %q, want eslint (not fallback)", failures[0].Framework)
	}
}
