package parser

import "strings"

// stripTimestamp removes a GitHub Actions timestamp prefix from a log line.
// e.g. "2026-03-20T12:15:15.1234567Z   content" → "  content"
func stripTimestamp(line string) string {
	if len(line) < 28 {
		return line
	}
	// Quick check: timestamps start with a digit and contain 'T' and 'Z'.
	if line[0] < '0' || line[0] > '9' {
		return line
	}
	// Find the 'Z' that ends the timestamp.
	idx := strings.IndexByte(line, 'Z')
	if idx < 20 || idx > 35 {
		return line
	}
	return line[idx+1:]
}

// Failure represents a single test failure extracted from CI logs.
type Failure struct {
	TestName  string `json:"test_name"`
	Message   string `json:"message"`
	Location  string `json:"location,omitempty"`
	Duration  string `json:"duration,omitempty"`
	Framework string `json:"framework"`
}

// FrameworkParser detects and extracts failures from CI logs.
type FrameworkParser interface {
	Name() string
	Detect(logs string) bool
	Extract(logs string) []Failure
}

// parsers is the ordered list of framework parsers.
// First match wins; fallback fires if none match.
var parsers = []FrameworkParser{
	&DotnetParser{},
	&GoParser{},
	&VitestParser{},
	&PythonParser{},
}

// Parse runs auto-detection against the logs and extracts failures.
// Returns failures from the first matching parser, or falls back to
// last-N-lines extraction if no parser matches.
func Parse(logs string) []Failure {
	for _, p := range parsers {
		if p.Detect(logs) {
			return p.Extract(logs)
		}
	}
	return (&FallbackParser{}).Extract(logs)
}

// DetectFramework returns the name of the detected framework, or "unknown".
func DetectFramework(logs string) string {
	for _, p := range parsers {
		if p.Detect(logs) {
			return p.Name()
		}
	}
	return "unknown"
}
