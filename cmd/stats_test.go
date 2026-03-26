package cmd

import (
	"slices"
	"testing"

	"github.com/seanhalberthal/gh-bench/internal/runner"
)

func TestParseRunIDs(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []int64
		wantErr bool
	}{
		{"comma separated", "100,200,300", []int64{100, 200, 300}, false},
		{"with spaces", "100, 200, 300", []int64{100, 200, 300}, false},
		{"single", "42", []int64{42}, false},
		{"trailing comma", "100,200,", []int64{100, 200}, false},
		{"empty string", "", nil, false},
		{"invalid id", "100,abc", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRunIDs(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseRunIDs(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && !slices.Equal(got, tt.want) {
				t.Errorf("parseRunIDs(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolvePatternFlag(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		preset  string
		wantErr bool
	}{
		{"pattern only", `(?P<value>\d+)`, "", false},
		{"preset only", "duration", "", false},
		{"both set", `(?P<value>\d+)`, "duration", true},
		{"neither set", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolvePatternFlag(tt.pattern, tt.preset)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolvePatternFlag(%q, %q) error = %v, wantErr = %v", tt.pattern, tt.preset, err, tt.wantErr)
			}
		})
	}
}

func TestStripCommonPrefix(t *testing.T) {
	tests := []struct {
		name   string
		labels []string
		want   []string
	}{
		{
			"shared path prefix",
			[]string{
				"github.com/foo/bar/internal/a",
				"github.com/foo/bar/internal/b",
			},
			[]string{"a", "b"},
		},
		{
			"single label unchanged",
			[]string{"github.com/foo/bar/internal/a"},
			[]string{"github.com/foo/bar/internal/a"},
		},
		{
			"no path separator",
			[]string{"abc", "abd"},
			[]string{"abc", "abd"},
		},
		{
			"short prefix not stripped",
			[]string{"a/x", "a/y"},
			[]string{"a/x", "a/y"},
		},
		{
			"no common prefix",
			[]string{"foo/bar", "baz/qux"},
			[]string{"foo/bar", "baz/qux"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := make(runner.ExtractedValues, len(tt.labels))
			for i, l := range tt.labels {
				values[i] = runner.ExtractedValue{Label: l}
			}
			got := stripCommonPrefix(values)
			if !slices.Equal(got, tt.want) {
				t.Errorf("stripCommonPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		max   int
		want  string
	}{
		{"shorter than max", "hello", 10, "hello"},
		{"exactly max", "hello", 5, "hello"},
		{"longer than max", "hello world", 8, "hello w…"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.max)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
			}
		})
	}
}
