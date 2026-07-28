// Package cli implements the opencli command-line interface.
package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

// opencli generates itself! like an ouroboros!
//go:generate go run .. generate go-cobra ../opencli.yaml --output . --package cli

// ErrInvalid is returned when a specification fails validation. The validate
// command prints its own diagnostics, so callers should treat this as a bare
// "exit non-zero" signal rather than something to print.
var ErrInvalid = errors.New("specification is invalid")

func newRootCmd() *cobra.Command {
	validateCommand := NewValidateCommand(validateHandler)
	generateCommand := NewGenerateCommand(generateGoCobraHandler, generateRustClapHandler)
	cmd := NewRootCommand(validateCommand, generateCommand)
	// Subcommands report their own errors; don't let Cobra echo usage on failure.
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd
}

// Execute runs the opencli root command.
func Execute() error {
	return newRootCmd().Execute()
}
