package cli

import (
	"github.com/spf13/cobra"
)

func NewRootCommand(
	validate ValidateCommand,
	generate GenerateCommand,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "opencli",
		Short: "Work with OpenCLI specifications.",
		Long: `OpenCLI validates OpenCLI specification documents and generates typed
command scaffolding for supported language and framework pairings.
`,
	}

	cmd.AddCommand(
		(*cobra.Command)(validate),
		(*cobra.Command)(generate),
	)

	return cmd
}
