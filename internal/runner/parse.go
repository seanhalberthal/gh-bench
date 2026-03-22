package runner

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Preset defines a reusable extraction pattern with metadata.
type Preset struct {
	Pattern     string
	Description string
	Example     string
}

// Presets maps preset names to their definitions.
var Presets = map[string]Preset{
	"duration": {
		Pattern:     `(?i)(?:took|duration|time|elapsed|finished in|completed in)\s*[:=]?\s*(?P<duration>[0-9.]+)\s*(?:ms|s)\b`,
		Description: "Common duration/timing output",
		Example:     "Took 12.5s, duration: 45ms, elapsed: 3.2s",
	},
	"coverage": {
		Pattern:     `(?i)coverage\s*[:=]?\s*(?P<coverage>[0-9.]+)\s*%`,
		Description: "Test coverage percentages",
		Example:     "Coverage: 85.2%, coverage=91%",
	},
	"go-test": {
		Pattern:     `^ok\s+(?P<label>\S+)\s+(?P<duration>[0-9.]+)s`,
		Description: "Go test package duration (labelled by package)",
		Example:     "ok  github.com/foo/bar  1.234s",
	},
	"jest": {
		Pattern:     `(?i)(?:time|duration)\s*:?\s+(?P<duration>[0-9.]+)\s*(?:ms|s)\b`,
		Description: "Jest/Vitest test suite time",
		Example:     "Time:        4.589 s",
	},
	"pytest": {
		Pattern:     `(?i)passed.*?\bin\s+(?P<duration>[0-9.]+)s`,
		Description: "Pytest suite duration",
		Example:     "42 passed in 3.45s",
	},
	"bundle-size": {
		Pattern:     `(?i)(?:bundle[- ]?size|size)\s*[:=]?\s*(?P<size>[0-9.]+)\s*(?:kb|mb|gb|bytes|b)\b`,
		Description: "Bundle or file size output",
		Example:     "Bundle size: 245.3 kB, size: 1.2 MB",
	},
}

// PresetNames returns sorted preset names.
func PresetNames() []string {
	names := make([]string, 0, len(Presets))
	for name := range Presets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ResolvePattern returns the regex for a preset name, or an error if not found.
func ResolvePattern(name string) (string, error) {
	p, ok := Presets[name]
	if !ok {
		return "", fmt.Errorf("unknown preset %q (available: %s)", name, strings.Join(PresetNames(), ", "))
	}
	return p.Pattern, nil
}

// ExtractedValue holds a single extracted value from a run's log.
type ExtractedValue struct {
	RunID int64
	Title string
	Label string // populated when pattern contains (?P<label>...)
	Raw   string
	Value float64
}

// ExtractedValues is a slice of ExtractedValue.
type ExtractedValues []ExtractedValue

// Numbers returns just the numeric values.
func (ev ExtractedValues) Numbers() []float64 {
	nums := make([]float64, len(ev))
	for i, v := range ev {
		nums[i] = v.Value
	}
	return nums
}

// HasLabels returns true if any value has a label.
func (ev ExtractedValues) HasLabels() bool {
	for _, v := range ev {
		if v.Label != "" {
			return true
		}
	}
	return false
}

// CompilePattern validates and compiles an extraction pattern.
// Returns the compiled regex, the value group index, and the label group index
// (-1 if no (?P<label>...) group is present).
func CompilePattern(pattern string) (*regexp.Regexp, int, int, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, 0, -1, fmt.Errorf("compiling pattern %q: %w", pattern, err)
	}

	// Find the first named group that isn't "label" — that's the value group.
	valueGroup := ""
	for _, name := range re.SubexpNames() {
		if name != "" && name != "label" {
			valueGroup = name
			break
		}
	}
	if valueGroup == "" {
		return nil, 0, -1, fmt.Errorf("pattern %q must contain at least one named capture group (?P<name>...) for numeric extraction", pattern)
	}

	labelIdx := re.SubexpIndex("label") // -1 if not present

	return re, re.SubexpIndex(valueGroup), labelIdx, nil
}

// matchResult holds a single regex match with optional label.
type matchResult struct {
	value string
	label string
}

// ExtractValues applies a pre-compiled regex to each run's log
// and extracts numeric values. When matchAll is true, all matches per
// run are returned (not just the first). labelIdx is the index of the
// optional (?P<label>...) group (-1 if absent).
func ExtractValues(results []RunResult, re *regexp.Regexp, groupIdx, labelIdx int, matchAll bool) (ExtractedValues, error) {
	var values ExtractedValues
	for _, r := range results {
		var matches []matchResult
		if matchAll {
			matches = findAllMatches(re, r.Log, groupIdx, labelIdx)
		} else {
			if m, ok := findFirstMatch(re, r.Log, groupIdx, labelIdx); ok {
				matches = []matchResult{m}
			}
		}

		for _, match := range matches {
			num, err := strconv.ParseFloat(match.value, 64)
			if err != nil {
				return nil, fmt.Errorf("value %q from run %d is not numeric: %w", match.value, r.RunID, err)
			}
			values = append(values, ExtractedValue{
				RunID: r.RunID,
				Title: r.Title,
				Label: match.label,
				Raw:   match.value,
				Value: num,
			})
		}
	}

	return values, nil
}

// findFirstMatch finds the first match of the regex in the log.
func findFirstMatch(re *regexp.Regexp, log string, groupIdx, labelIdx int) (matchResult, bool) {
	for line := range strings.SplitSeq(log, "\n") {
		match := re.FindStringSubmatch(line)
		if match != nil && groupIdx < len(match) {
			mr := matchResult{value: match[groupIdx]}
			if labelIdx >= 0 && labelIdx < len(match) {
				mr.label = match[labelIdx]
			}
			return mr, true
		}
	}
	return matchResult{}, false
}

// findAllMatches returns every match of the regex's named group across all lines.
func findAllMatches(re *regexp.Regexp, log string, groupIdx, labelIdx int) []matchResult {
	var results []matchResult
	for line := range strings.SplitSeq(log, "\n") {
		match := re.FindStringSubmatch(line)
		if match != nil && groupIdx < len(match) {
			mr := matchResult{value: match[groupIdx]}
			if labelIdx >= 0 && labelIdx < len(match) {
				mr.label = match[labelIdx]
			}
			results = append(results, mr)
		}
	}
	return results
}
