package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Filename is the expected config file name.
const Filename = ".gh-bench.yml"

// Config holds project-level defaults for gh-bench commands.
type Config struct {
	Workflow string         `yaml:"workflow"`
	Failures FailuresConfig `yaml:"failures"`
}

// FailuresConfig holds defaults for the failures subcommand.
type FailuresConfig struct {
	ExcludeSteps []string `yaml:"exclude-steps"`
}

// Load reads the config file from the current directory, walking up to the
// git root. Returns a zero Config (not an error) if no file is found.
func Load() (Config, error) {
	path, err := find()
	if err != nil {
		return Config{}, nil // no config file — use defaults
	}
	return readFile(path)
}

// find walks up from the working directory looking for Filename, stopping
// at the filesystem root or a .git directory.
func find() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		candidate := filepath.Join(dir, Filename)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		// Stop at repo root if present.
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // filesystem root
		}
		dir = parent
	}

	return "", errors.New("config file not found")
}

// readFile parses a YAML config file.
func readFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
