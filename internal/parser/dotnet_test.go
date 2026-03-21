package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func readTestData(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading testdata/%s: %v", name, err)
	}
	return string(data)
}

func TestDotnetParser_Detect_XUnit(t *testing.T) {
	logs := readTestData(t, "dotnet_xunit.txt")
	p := &DotnetParser{}
	if !p.Detect(logs) {
		t.Error("expected Detect to return true for xUnit output")
	}
}

func TestDotnetParser_Detect_NUnit(t *testing.T) {
	logs := readTestData(t, "dotnet_nunit.txt")
	p := &DotnetParser{}
	if !p.Detect(logs) {
		t.Error("expected Detect to return true for NUnit output")
	}
}

func TestDotnetParser_Detect_NoMatch(t *testing.T) {
	p := &DotnetParser{}
	if p.Detect("just some random log output") {
		t.Error("expected Detect to return false for unrelated logs")
	}
}

func TestDotnetParser_Extract_XUnit(t *testing.T) {
	logs := readTestData(t, "dotnet_xunit.txt")
	p := &DotnetParser{}
	failures := p.Extract(logs)

	if len(failures) != 2 {
		t.Fatalf("expected 2 failures, got %d", len(failures))
	}

	f1 := failures[0]
	if f1.TestName != "Dana.Tests.Search.GlobalSearchTests.Returns_Results_For_Partial_Match" {
		t.Errorf("unexpected test name: %s", f1.TestName)
	}
	if f1.Duration != "45ms" {
		t.Errorf("unexpected duration: %s", f1.Duration)
	}
	if f1.Framework != "xUnit" {
		t.Errorf("unexpected framework: %s", f1.Framework)
	}
	if f1.Location == "" {
		t.Error("expected a location to be extracted")
	}

	f2 := failures[1]
	if f2.TestName != "Dana.Tests.Search.GlobalSearchTests.Excludes_Hidden_Participants" {
		t.Errorf("unexpected test name: %s", f2.TestName)
	}
	if f2.Duration != "12ms" {
		t.Errorf("unexpected duration: %s", f2.Duration)
	}
}

func TestDotnetParser_Extract_NUnit(t *testing.T) {
	logs := readTestData(t, "dotnet_nunit.txt")
	p := &DotnetParser{}
	failures := p.Extract(logs)

	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(failures))
	}

	if failures[0].TestName != "Dana.Tests.Data.MigrationTests.Can_Apply_All_Migrations" {
		t.Errorf("unexpected test name: %s", failures[0].TestName)
	}
}

func TestDotnetParser_Extract_EmptyLogs(t *testing.T) {
	p := &DotnetParser{}
	failures := p.Extract("")
	if len(failures) != 0 {
		t.Errorf("expected 0 failures for empty logs, got %d", len(failures))
	}
}

func TestDotnetParser_Name(t *testing.T) {
	p := &DotnetParser{}
	if p.Name() != "xUnit" {
		t.Errorf("expected name 'xUnit', got %q", p.Name())
	}
}
