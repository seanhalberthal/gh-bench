package parser

import (
	"os"
	"path/filepath"
	"strings"
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
	if f1.Framework != "dotnet" {
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
	if p.Name() != "dotnet" {
		t.Errorf("expected name 'dotnet', got %q", p.Name())
	}
}

func TestDotnetParser_Detect_Summary(t *testing.T) {
	logs := readTestData(t, "dotnet_summary_only.txt")
	p := &DotnetParser{}
	if !p.Detect(logs) {
		t.Error("expected Detect to return true for dotnet summary output")
	}
}

func TestDotnetParser_Extract_SummaryOnly(t *testing.T) {
	logs := readTestData(t, "dotnet_summary_only.txt")
	p := &DotnetParser{}
	failures := p.Extract(logs)

	if len(failures) != 1 {
		t.Fatalf("expected 1 summary failure, got %d", len(failures))
	}

	f := failures[0]
	if f.TestName != "(test run failed)" {
		t.Errorf("unexpected test name: %s", f.TestName)
	}
	if f.Framework != "dotnet" {
		t.Errorf("unexpected framework: %s", f.Framework)
	}
	if !strings.Contains(f.Message, "Failed") {
		t.Errorf("expected message to contain 'Failed', got: %s", f.Message)
	}
}

func TestDotnetParser_Extract_CI_WithTimestamps(t *testing.T) {
	logs := readTestData(t, "dotnet_ci.txt")
	p := &DotnetParser{}

	if !p.Detect(logs) {
		t.Fatal("expected Detect to return true for CI output with timestamps")
	}

	failures := p.Extract(logs)
	if len(failures) != 2 {
		t.Fatalf("expected 2 failures, got %d", len(failures))
	}

	f1 := failures[0]
	if f1.TestName != "Dana.Tests.Hearings.CreateHearingTests.Chambersadmin_Can_Create_Hearing" {
		t.Errorf("unexpected test name: %s", f1.TestName)
	}
	if f1.Duration != "156ms" {
		t.Errorf("unexpected duration: %s", f1.Duration)
	}
	if !strings.Contains(f1.Message, "403") {
		t.Errorf("expected message to contain '403', got: %s", f1.Message)
	}

	f2 := failures[1]
	if f2.TestName != "Dana.Tests.Hearings.CreateHearingTests.Returns_Validation_Error_For_Missing_Fields" {
		t.Errorf("unexpected test name: %s", f2.TestName)
	}
}

func TestDotnetParser_Extract_ParameterisedTestWithSpaces(t *testing.T) {
	logs := readTestData(t, "dotnet_shouldly.txt")
	p := &DotnetParser{}

	if !p.Detect(logs) {
		t.Fatal("expected Detect to return true")
	}

	failures := p.Extract(logs)
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(failures))
	}

	f := failures[0]

	// Full parameterised test name must be captured.
	if !strings.HasPrefix(f.TestName, "Dana.IntegrationTests.TokenManagement") {
		t.Errorf("unexpected test name: %s", f.TestName)
	}
	if !strings.Contains(f.TestName, "Microsoft 365") {
		t.Errorf("expected test name to include parameter values, got: %s", f.TestName)
	}

	// Duration with space: "1 s"
	if f.Duration != "1 s" {
		t.Errorf("unexpected duration: %q", f.Duration)
	}

	// Should extract Shouldly assertion message.
	if !strings.Contains(f.Message, "should start with") {
		t.Errorf("expected Shouldly assertion in message, got: %s", f.Message)
	}

	// Should extract location.
	if f.Location == "" {
		t.Error("expected location to be extracted")
	}
	if !strings.Contains(f.Location, "ExchangeCodeForTokensIntegrationTests.cs:line 181") {
		t.Errorf("unexpected location: %s", f.Location)
	}
}
