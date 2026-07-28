package cli

import (
	"context"
	"github.com/spf13/cobra"
)

type ValidateParams struct {
	Spec string
}
type ValidateHandler func(ctx context.Context, cmd *cobra.Command, io IO, p ValidateParams) error

func NewValidateCommand(validate ValidateHandler) ValidateCommand {
	cmd := &cobra.Command{
		Use:   "validate [spec]",
		Short: "Validate an OpenCLI specification against the OpenCLI schema.",
		Long: `Validate reads an OpenCLI specification in JSON or YAML and checks it
structurally against the embedded schema, then semantically for issues
such as dangling references, argument ordering, and identifier collisions.

Pass - or omit the path to read from stdin.
`,
		Args: rangeArgs(0, 1),
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		var rawSpec string
		if len(args) > 0 {
			rawSpec = args[0]
		}

		p := ValidateParams{
			Spec: rawSpec,
		}

		return validate(cmd.Context(), cmd, newIO(cmd), p)
	}

	return ValidateCommand(cmd)
}

type ValidateCommand *cobra.Command
