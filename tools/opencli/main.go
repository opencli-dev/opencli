package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/opencli-dev/opencli/tools/opencli/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		// ErrInvalid means the validate command already printed diagnostics;
		// just exit non-zero. Anything else is an operational error worth showing.
		if !errors.Is(err, cli.ErrInvalid) {
			fmt.Fprintln(os.Stderr, "opencli:", err)
		}
		os.Exit(1)
	}
}
