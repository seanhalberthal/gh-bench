package main

import (
	"os"

	"github.com/undont/gh-bench/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
