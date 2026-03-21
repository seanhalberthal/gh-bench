package parser

import (
	"regexp"
	"strings"
)

const fallbackLines = 30

// FallbackParser extracts error-relevant lines from a log when no framework is detected.
type FallbackParser struct{}

// Name returns the parser name.
func (f *FallbackParser) Name() string { return "unknown" }

// Detect always returns false — fallback is used when no other parser matches.
func (f *FallbackParser) Detect(logs string) bool { return false }

var (
	// GitHub Actions annotation lines.
	ghAnnotationRe = regexp.MustCompile(`^##\[(group|endgroup|warning|notice|debug)\]`)
	// Environment variable block lines: "  KEY: value"
	envVarRe = regexp.MustCompile(`^\s{2,}[A-Z][A-Z0-9_]+:\s+\S`)
	// Shell/interpreter lines.
	shellLineRe = regexp.MustCompile(`^\s*shell:\s+/`)
	// ##[error] lines — these carry the actual signal.
	ghErrorRe = regexp.MustCompile(`^##\[error\](.+)`)
	// Shell script source lines (expanded by set -x or ##[group] in GitHub Actions).
	shellScriptRe = regexp.MustCompile(`^(if\s+\[|then$|else$|fi$|done$|do$|exit\s+\d|echo\s+"[^"]*"$|#\s)`)
	// ANSI colour code escape sequences (e.g. [36;1m...[0m).
	ansiEscapeRe = regexp.MustCompile(`^\[[\d;]+m`)
	// GitHub Actions runner commands: [command]/usr/bin/git ...
	ghCommandRe = regexp.MustCompile(`^\[command\]`)
	// GitHub Actions cleanup/lifecycle lines.
	ghCleanupRe = regexp.MustCompile(`^(Cleaning up orphan processes|Terminate orphan process:|Removing |Prepare all required actions|Complete job name:)`)
)

// isBoilerplate returns true for lines that are GitHub Actions infrastructure
// rather than actual error output. The line should already have timestamps stripped.
func isBoilerplate(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || trimmed == "env:" {
		return true
	}
	if ghAnnotationRe.MatchString(trimmed) {
		return true
	}
	if envVarRe.MatchString(line) {
		return true
	}
	if shellLineRe.MatchString(trimmed) {
		return true
	}
	if shellScriptRe.MatchString(trimmed) {
		return true
	}
	if ansiEscapeRe.MatchString(trimmed) {
		return true
	}
	if ghCommandRe.MatchString(trimmed) {
		return true
	}
	if ghCleanupRe.MatchString(trimmed) {
		return true
	}
	// Generic GitHub Actions error that carries no information.
	if strings.HasPrefix(trimmed, "##[error]Process completed with exit code") {
		return true
	}
	return false
}

// Extract returns error-relevant lines from the log tail.
// It strips GitHub Actions timestamps and boilerplate (env vars, annotations,
// shell script source) and prioritises ##[error] messages.
func (f *FallbackParser) Extract(logs string) []Failure {
	if strings.TrimSpace(logs) == "" {
		return nil
	}

	rawLines := strings.Split(logs, "\n")

	// Strip timestamps from all lines up front.
	cleaned := make([]string, len(rawLines))
	for i, line := range rawLines {
		cleaned[i] = stripTimestamp(line)
	}

	// Collect ##[error] lines — these are the primary signal.
	// Filter out the generic "Process completed with exit code N." which
	// GitHub Actions always adds and carries no useful information.
	var errorMsgs []string
	for _, line := range cleaned {
		trimmed := strings.TrimSpace(line)
		if m := ghErrorRe.FindStringSubmatch(trimmed); m != nil {
			msg := strings.TrimSpace(m[1])
			if !strings.HasPrefix(msg, "Process completed with exit code") {
				errorMsgs = append(errorMsgs, msg)
			}
		}
	}

	// Build the tail, filtering out boilerplate.
	var filtered []string
	for _, line := range cleaned {
		if !isBoilerplate(line) {
			filtered = append(filtered, strings.TrimSpace(line))
		}
	}

	// Take the last N non-boilerplate lines.
	start := 0
	if len(filtered) > fallbackLines {
		start = len(filtered) - fallbackLines
	}
	tail := strings.TrimSpace(strings.Join(filtered[start:], "\n"))

	// If we found meaningful ##[error] messages, lead with those.
	msg := tail
	if len(errorMsgs) > 0 {
		msg = strings.Join(errorMsgs, "\n")
		// Only append log tail if the error is something actionable.
		// Cancellations and timeouts don't benefit from log context.
		isCancellation := len(errorMsgs) == 1 && (errorMsgs[0] == "The operation was canceled." || strings.Contains(errorMsgs[0], "cancelled") || strings.Contains(errorMsgs[0], "timed out"))
		if !isCancellation && tail != "" && tail != msg {
			msg += "\n\n--- log tail ---\n" + tail
		}
	}

	if msg == "" {
		return nil
	}

	return []Failure{
		{
			TestName:  "(unstructured output)",
			Message:   msg,
			Framework: "unknown",
		},
	}
}
