package parser

import "strings"

const fallbackLines = 30

// FallbackParser extracts the last N lines of a log when no framework is detected.
type FallbackParser struct{}

// Name returns the parser name.
func (f *FallbackParser) Name() string { return "unknown" }

// Detect always returns false — fallback is used when no other parser matches.
func (f *FallbackParser) Detect(logs string) bool { return false }

// Extract returns the last N lines as a single failure entry.
func (f *FallbackParser) Extract(logs string) []Failure {
	if strings.TrimSpace(logs) == "" {
		return nil
	}

	lines := strings.Split(logs, "\n")
	start := 0
	if len(lines) > fallbackLines {
		start = len(lines) - fallbackLines
	}

	tail := strings.Join(lines[start:], "\n")
	return []Failure{
		{
			TestName:  "(unstructured output)",
			Message:   strings.TrimSpace(tail),
			Framework: "unknown",
		},
	}
}
