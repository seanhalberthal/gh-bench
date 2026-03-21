package main

import (
	"os"

	"github.com/seanhalberthal/gh-bench/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
