package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadFile_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, Filename)
	content := `
workflow: ci.yml
failures:
  exclude-steps:
    - "Check CI status"
    - "Summary"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := readFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Workflow != "ci.yml" {
		t.Errorf("Workflow = %q, want %q", cfg.Workflow, "ci.yml")
	}
	if len(cfg.Failures.ExcludeSteps) != 2 {
		t.Fatalf("ExcludeSteps len = %d, want 2", len(cfg.Failures.ExcludeSteps))
	}
	if cfg.Failures.ExcludeSteps[0] != "Check CI status" {
		t.Errorf("ExcludeSteps[0] = %q, want %q", cfg.Failures.ExcludeSteps[0], "Check CI status")
	}
}

func TestReadFile_Missing(t *testing.T) {
	cfg, err := readFile("/nonexistent/.gh-bench.yml")
	if err != nil {
		t.Fatalf("unexpected error for missing file: %v", err)
	}
	if cfg.Workflow != "" {
		t.Errorf("expected zero Config, got workflow=%q", cfg.Workflow)
	}
}

func TestReadFile_Empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, Filename)
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := readFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Workflow != "" {
		t.Errorf("expected zero Config for empty file")
	}
}

func TestReadFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, Filename)
	if err := os.WriteFile(path, []byte(":\n\t:::bad"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := readFile(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestReadFile_WorkflowOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, Filename)
	if err := os.WriteFile(path, []byte("workflow: deploy.yml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := readFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Workflow != "deploy.yml" {
		t.Errorf("Workflow = %q, want %q", cfg.Workflow, "deploy.yml")
	}
	if len(cfg.Failures.ExcludeSteps) != 0 {
		t.Errorf("expected no exclude steps, got %d", len(cfg.Failures.ExcludeSteps))
	}
}

func TestFind_InCurrentDir(t *testing.T) {
	dir := t.TempDir()

	// Create .git to act as repo root boundary.
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, Filename), []byte("workflow: ci.yml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(dir)

	path, err := find()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(path) != Filename {
		t.Errorf("found %q, want %q", path, Filename)
	}
}

func TestFind_WalksUp(t *testing.T) {
	root := t.TempDir()
	// Resolve symlinks (macOS /var -> /private/var).
	root, _ = filepath.EvalSymlinks(root)

	// Config at root, working dir is a subdirectory.
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, Filename), []byte("workflow: ci.yml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "src", "pkg")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Chdir(sub)

	path, err := find()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Dir(path) != root {
		t.Errorf("found config at %q, expected it at root %q", path, root)
	}
}

func TestFind_StopsAtGitRoot(t *testing.T) {
	root := t.TempDir()

	// .git boundary but no config file — should not be found.
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Chdir(root)

	_, err := find()
	if err == nil {
		t.Fatal("expected error when no config found")
	}
}
