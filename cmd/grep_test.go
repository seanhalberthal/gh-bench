package cmd

import (
	"strings"
	"testing"
)

func TestCompileMatcher_Substring(t *testing.T) {
	match, err := compileMatcher("hello", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		line string
		want bool
	}{
		{"hello world", true},
		{"HELLO WORLD", true},
		{"say Hello there", true},
		{"goodbye world", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := match(tt.line); got != tt.want {
			t.Errorf("match(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestCompileMatcher_Regex(t *testing.T) {
	match, err := compileMatcher(`error\s+\d+`, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		line string
		want bool
	}{
		{"error 404", true},
		{"ERROR 500", true},
		{"error: not found", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := match(tt.line); got != tt.want {
			t.Errorf("match(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestCompileMatcher_InvalidRegex(t *testing.T) {
	_, err := compileMatcher(`[invalid`, true)
	if err == nil {
		t.Error("expected error for invalid regex, got nil")
	}
}

func TestSearchLog_Basic(t *testing.T) {
	log := "line one\nfind me here\nline three\nfind me again\nline five"
	match, _ := compileMatcher("find me", false)

	matches := searchLog(log, match, 0, 0)

	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	if matches[0].Line != 2 {
		t.Errorf("matches[0].Line = %d, want 2", matches[0].Line)
	}
	if matches[0].Content != "find me here" {
		t.Errorf("matches[0].Content = %q, want %q", matches[0].Content, "find me here")
	}
	if matches[1].Line != 4 {
		t.Errorf("matches[1].Line = %d, want 4", matches[1].Line)
	}
}

func TestSearchLog_MaxMatches(t *testing.T) {
	log := "match\nmatch\nmatch\nmatch\nmatch"
	match, _ := compileMatcher("match", false)

	matches := searchLog(log, match, 0, 2)

	if len(matches) != 2 {
		t.Fatalf("expected 2 matches (max), got %d", len(matches))
	}
}

func TestSearchLog_Context(t *testing.T) {
	log := "line 1\nline 2\ntarget\nline 4\nline 5"
	match, _ := compileMatcher("target", false)

	matches := searchLog(log, match, 1, 0)

	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	// Context should include 1 line before and after.
	want := "line 2\ntarget\nline 4"
	if matches[0].Context != want {
		t.Errorf("matches[0].Context = %q, want %q", matches[0].Context, want)
	}
}

func TestSearchLog_ContextAtBoundaries(t *testing.T) {
	log := "target\nline 2\nline 3"
	match, _ := compileMatcher("target", false)

	matches := searchLog(log, match, 2, 0)

	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	// Context at start of log — no lines before, 2 lines after.
	want := "target\nline 2\nline 3"
	if matches[0].Context != want {
		t.Errorf("matches[0].Context = %q, want %q", matches[0].Context, want)
	}
}

func TestSearchLog_NoMatches(t *testing.T) {
	log := "line one\nline two\nline three"
	match, _ := compileMatcher("nonexistent", false)

	matches := searchLog(log, match, 0, 0)

	if len(matches) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(matches))
	}
}

func TestPrintGrepText(t *testing.T) {
	results := []grepResult{
		{
			RunID:  123,
			Title:  "Test Run",
			Date:   "2026-01-05",
			Branch: "main",
			Matches: []grepMatch{
				{Line: 10, Content: "error: something failed"},
				{Line: 25, Content: "error: another failure"},
			},
		},
	}

	// Capture stdout by redirecting — just verify it doesn't error.
	err := printGrepText(results, 2, 5)
	if err != nil {
		t.Fatalf("printGrepText error: %v", err)
	}
}

func TestPrintGrepJSON(t *testing.T) {
	results := []grepResult{
		{
			RunID:  456,
			Title:  "Build",
			Date:   "2026-03-20",
			Branch: "feat-x",
			Step:   "Run tests",
			Matches: []grepMatch{
				{Line: 5, Content: "FAIL: TestFoo"},
			},
		},
	}

	err := printGrepJSON(results)
	if err != nil {
		t.Fatalf("printGrepJSON error: %v", err)
	}
}

func TestPrintGrepCSV(t *testing.T) {
	results := []grepResult{
		{
			RunID:  789,
			Title:  "CI",
			Date:   "2026-06-15",
			Branch: "develop",
			Matches: []grepMatch{
				{Line: 1, Content: "test output"},
			},
		},
	}

	err := printGrepCSV(results)
	if err != nil {
		t.Fatalf("printGrepCSV error: %v", err)
	}
}

func TestSearchLog_EmptyLog(t *testing.T) {
	match, _ := compileMatcher("anything", false)

	matches := searchLog("", match, 0, 0)

	if len(matches) != 0 {
		t.Fatalf("expected 0 matches for empty log, got %d", len(matches))
	}
}

func TestGrepResult_TextWithStep(t *testing.T) {
	results := []grepResult{
		{
			RunID:  100,
			Title:  "Build",
			Date:   "2026-01-01",
			Branch: "main",
			Step:   "Run unit tests",
			Matches: []grepMatch{
				{Line: 1, Content: "FAIL"},
			},
		},
	}

	// Verify text output includes step name — just check no error.
	err := printGrepText(results, 1, 1)
	if err != nil {
		t.Fatalf("printGrepText error: %v", err)
	}
}

func TestGrepResult_TextWithContext(t *testing.T) {
	results := []grepResult{
		{
			RunID:  100,
			Title:  "Build",
			Date:   "2026-01-01",
			Matches: []grepMatch{
				{
					Line:    5,
					Content: "error here",
					Context: "line before\nerror here\nline after",
				},
			},
		},
	}

	err := printGrepText(results, 1, 1)
	if err != nil {
		t.Fatalf("printGrepText error: %v", err)
	}
}

func TestSearchLog_RegexMatch(t *testing.T) {
	log := "duration: 1234ms\nstatus: ok\nduration: 5678ms"
	match, _ := compileMatcher(`duration:\s+\d+ms`, true)

	matches := searchLog(log, match, 0, 0)

	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	if !strings.Contains(matches[0].Content, "1234ms") {
		t.Errorf("expected first match to contain 1234ms, got %q", matches[0].Content)
	}
}
