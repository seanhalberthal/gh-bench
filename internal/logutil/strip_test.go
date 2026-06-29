package logutil

import "testing"

func TestStripTimestamp(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"standard timestamp with space",
			"2026-03-16T13:34:37.3465175Z ok  \tgithub.com/foo/bar\t1.234s",
			"ok  \tgithub.com/foo/bar\t1.234s",
		},
		{
			"short fractional seconds",
			"2026-03-16T13:34:37.3Z some output",
			"some output",
		},
		{
			"timestamp without trailing space",
			"2026-03-20T12:15:15.1234567Z  content",
			" content",
		},
		{
			"no timestamp",
			"ok  \tgithub.com/foo/bar\t1.234s",
			"ok  \tgithub.com/foo/bar\t1.234s",
		},
		{
			"empty line",
			"",
			"",
		},
		{
			"too short",
			"2026-03",
			"2026-03",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripTimestamp(tt.input)
			if got != tt.want {
				t.Errorf("StripTimestamp(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripTimestamps(t *testing.T) {
	input := "2026-03-16T13:34:37.3465175Z line one\n2026-03-16T13:34:37.3558618Z line two\nno timestamp"
	got := StripTimestamps(input)
	want := "line one\nline two\nno timestamp"
	if got != want {
		t.Errorf("StripTimestamps() = %q, want %q", got, want)
	}
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"sgr colour codes", "\x1b[36;1mmake lint\x1b[0m", "make lint"},
		{"reset only", "done\x1b[0m", "done"},
		{"erase line", "progress\x1b[2K", "progress"},
		{"cursor move", "\x1b[1Gline", "line"},
		{"no ansi", "plain text", "plain text"},
		{"interleaved", "\x1b[31m✖\x1b[39m 1 problem", "✖ 1 problem"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripANSI(tt.input); got != tt.want {
				t.Errorf("StripANSI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
