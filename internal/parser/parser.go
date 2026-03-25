package parser

// Failure represents a single test failure extracted from CI logs.
type Failure struct {
	TestName  string `json:"test_name"`
	Message   string `json:"message"`
	Location  string `json:"location,omitempty"`
	Duration  string `json:"duration,omitempty"`
	Framework string `json:"framework"`
	Timestamp string `json:"timestamp,omitempty"`
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
