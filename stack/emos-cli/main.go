package main

import (
	"os"

	"github.com/automatika-robotics/emos-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
