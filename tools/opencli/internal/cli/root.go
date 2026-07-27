// Package cli implements the opencli command-line interface.
package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

// ErrInvalid is returned when a specification fails validation. The validate
// command prints its own diagnostics, so callers should treat this as a bare
// "exit non-zero" signal rather than something to print.
var ErrInvalid = errors.New("specification is invalid")

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "opencli",
		Short: "Work with OpenCLI specifications",
		Long:  "opencli reads and validates OpenCLI specification documents against the OpenCLI schema.",
		// Subcommands report their own errors; don't let Cobra echo usage on failure.
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(newValidateCmd())
	cmd.AddCommand(newGenerateCmd())

	return cmd
}

// Execute runs the opencli root command.
func Execute() error {
	return newRootCmd().Execute()
}
