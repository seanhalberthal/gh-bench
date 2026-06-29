package parser

import (
	"path/filepath"
	"regexp"
	"strings"
)

// GenericParser extracts findings from compiler and linter output that no
// framework-specific parser claims: eslint (stylish), tsc, golangci-lint,
// ruff/flake8, and gcc/clang style file:line:col diagnostics. It runs last,
// so the framework parsers always win when they detect.
type GenericParser struct{}

var (
	// eslint stylish summary, e.g. "✖ 3 problems (2 errors, 1 warning)".
	eslintSummaryRe = regexp.MustCompile(`✖\s+\d+\s+problem`)
	// eslint stylish file header: a bare path on its own line.
	fileHeaderRe = regexp.MustCompile(`^(\S+\.\w+)\s*$`)
	// eslint stylish row: "  23:54  error  message  rule".
	stylishRowRe = regexp.MustCompile(`^\s+(\d+):(\d+)\s+(error|warning)\s+(.+?)(?:\s{2,}(\S+))?\s*$`)

	// tsc default (non-tty): "file.ts(12,5): error TS2322: message".
	tscParenRe = regexp.MustCompile(`^(\S+\.\w+)\((\d+),(\d+)\):\s+(?:error|warning)\s+(TS\d+):\s+(.+)$`)
	// tsc pretty (tty): "file.ts:12:5 - error TS2322: message".
	tscColonRe = regexp.MustCompile(`^(\S+\.\w+):(\d+):(\d+)\s+-\s+(?:error|warning)\s+(TS\d+):\s+(.+)$`)
	// file:line[:col]: message — golangci-lint, ruff, gcc/clang, eslint unix.
	lineColonRe = regexp.MustCompile(`^(\S+\.\w+):(\d+)(?::(\d+))?:\s+(.+)$`)

	// trailing "(linter)" annotation used by golangci-lint.
	trailingRuleRe = regexp.MustCompile(`^(.*?)\s+\(([\w][\w./-]*)\)\s*$`)
	// leading severity used by gcc/clang/mypy.
	severityRe = regexp.MustCompile(`^(error|warning|note):\s+(.+)$`)
	// leading lint code used by ruff/flake8, e.g. "F401 ...".
	leadingCodeRe = regexp.MustCompile(`^([A-Z]{1,5}\d+)\s+(.+)$`)
)

// Name returns the parser name.
func (p *GenericParser) Name() string { return "compiler/linter" }

// Detect checks for eslint/tsc anchors or a cluster of file:line:col
// diagnostics. A single stray location reference is not enough.
func (p *GenericParser) Detect(logs string) bool {
	if eslintSummaryRe.MatchString(logs) {
		return true
	}
	n := 0
	for line := range strings.SplitSeq(logs, "\n") {
		if tscParenRe.MatchString(line) || tscColonRe.MatchString(line) {
			return true
		}
		if lineColonRe.MatchString(line) {
			if n++; n >= 2 {
				return true
			}
		}
	}
	return false
}

// Extract parses both eslint stylish blocks and per-line diagnostics.
func (p *GenericParser) Extract(logs string) []Failure {
	lines := strings.Split(logs, "\n")
	var out []Failure
	out = append(out, extractStylish(lines)...)
	out = append(out, extractLineOriented(lines)...)
	return out
}

// extractStylish parses eslint stylish output: a bare file path followed by
// indented "line:col severity message rule" rows.
func extractStylish(lines []string) []Failure {
	var out []Failure
	var currentFile string

	for _, line := range lines {
		if m := stylishRowRe.FindStringSubmatch(line); m != nil && currentFile != "" {
			loc := currentFile + ":" + m[1] + ":" + m[2]
			msg := strings.TrimSpace(m[4])
			if m[3] == "warning" {
				msg = "warning: " + msg
			}
			f := Failure{Framework: "eslint", Message: msg, Location: loc}
			if rule := m[5]; rule != "" {
				f.TestName = rule
			} else {
				f.TestName = loc
				f.Location = ""
			}
			out = append(out, f)
			continue
		}
		if fileHeaderRe.MatchString(line) {
			currentFile = strings.TrimSpace(line)
		}
	}

	return out
}

// extractLineOriented parses single-line file:line:col diagnostics.
func extractLineOriented(lines []string) []Failure {
	var out []Failure

	for _, line := range lines {
		if m := tscParenRe.FindStringSubmatch(line); m != nil {
			out = append(out, tscFinding(m))
			continue
		}
		if m := tscColonRe.FindStringSubmatch(line); m != nil {
			out = append(out, tscFinding(m))
			continue
		}
		m := lineColonRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		loc := m[1] + ":" + m[2]
		if m[3] != "" {
			loc += ":" + m[3]
		}
		out = append(out, buildLineFinding(m[1], loc, strings.TrimSpace(m[4])))
	}

	return out
}

// tscFinding builds a Failure from a tsc regex match (file, line, col, code, msg).
func tscFinding(m []string) Failure {
	return Failure{
		Framework: "tsc",
		TestName:  m[4],
		Message:   strings.TrimSpace(m[5]),
		Location:  m[1] + ":" + m[2] + ":" + m[3],
	}
}

// buildLineFinding decomposes the message tail of a file:line:col diagnostic
// into severity, code/rule, and message, then infers the tool from the file.
func buildLineFinding(file, loc, rest string) Failure {
	var severity, name string

	if m := severityRe.FindStringSubmatch(rest); m != nil {
		severity = m[1]
		rest = m[2]
	}
	if m := leadingCodeRe.FindStringSubmatch(rest); m != nil {
		name = m[1]
		rest = m[2]
	}
	if name == "" {
		if m := trailingRuleRe.FindStringSubmatch(rest); m != nil {
			rest = strings.TrimSpace(m[1])
			name = m[2]
		}
	}

	msg := rest
	if severity == "warning" || severity == "note" {
		msg = severity + ": " + msg
	}

	f := Failure{Framework: inferTool(file, name), Message: msg}
	if name != "" {
		f.TestName = name
		f.Location = loc
	} else {
		f.TestName = loc
	}
	return f
}

// inferTool guesses the tool from the file extension and whether a lint code
// was present.
func inferTool(file, code string) string {
	switch strings.ToLower(filepath.Ext(file)) {
	case ".go":
		return "golangci-lint"
	case ".py":
		if code != "" {
			return "ruff"
		}
		return "python"
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return "eslint"
	case ".c", ".cc", ".cpp", ".cxx", ".h", ".hpp":
		return "gcc/clang"
	}
	return "compiler/linter"
}
