package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

var root = &cobra.Command{
	Use:     "todoist-cli",
	Short:   "A fast, minimal Todoist terminal client",
	Version: version,
}

func main() {
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
