package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/opencli-dev/opencli/tools/opencli/internal/validate"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [spec]",
		Short: "Validate an OpenCLI specification against the OpenCLI schema",
		Long: "Validate reads an OpenCLI specification (JSON or YAML) from a file or stdin\n" +
			"and checks it against the embedded OpenCLI schema.\n\n" +
			"Pass '-' or omit the path to read from stdin.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "-"
			if len(args) == 1 {
				path = args[0]
			}

			data, src, err := readInput(path)
			if err != nil {
				return err
			}

			schema, err := validate.Load()
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if err := schema.Validate(data); err != nil {
				issues := validate.Issues(err)
				if issues == nil {
					// A parse/decode error rather than a schema violation.
					return err
				}
				fmt.Fprintf(out, "✗ %s is not a valid OpenCLI specification (%d issue(s))\n\n", src, len(issues))
				for _, iss := range issues {
					fmt.Fprintf(out, "  %s\n", iss)
				}
				return ErrInvalid
			}

			fmt.Fprintf(out, "✓ %s is a valid OpenCLI specification\n", src)
			return nil
		},
	}
}

func readInput(path string) (data []byte, source string, err error) {
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, "", fmt.Errorf("read stdin: %w", err)
		}
		return data, "<stdin>", nil
	}

	data, err = os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", path, err)
	}
	return data, path, nil
}
