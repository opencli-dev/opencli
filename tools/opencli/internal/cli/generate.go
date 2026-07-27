package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/opencli-dev/opencli/tools/opencli/internal/codegen"
	"github.com/opencli-dev/opencli/tools/opencli/validate"
)

func newGenerateCmd() *cobra.Command {
	var output, pkg string

	cmd := &cobra.Command{
		Use:   "generate [spec]",
		Short: "Generate Cobra command scaffolding from an OpenCLI specification",
		Long: "Generate reads an OpenCLI specification (JSON or YAML), validates it, and\n" +
			"emits Go source: one file per top-level command (a Handler func type, a\n" +
			"Params struct, and a constructor building the parsed *cobra.Command), plus\n" +
			"a shared runtime file and a root command assembler. Application code supplies\n" +
			"one Handler function per operationId; generated code only parses and\n" +
			"validates the command line before calling it.\n\n" +
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

			v, err := validate.New()
			if err != nil {
				return err
			}

			s, err := v.Check(data)
			if err != nil {
				var verr *validate.Error
				if !errors.As(err, &verr) {
					return err
				}
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "✗ %s is not a valid OpenCLI specification (%d issue(s))\n\n", src, len(verr.Issues))
				for _, iss := range verr.Issues {
					fmt.Fprintf(out, "  %s\n", iss)
				}
				return ErrInvalid
			}

			files, err := codegen.Generate(s, pkg)
			if err != nil {
				return fmt.Errorf("generate: %w", err)
			}

			if err := os.MkdirAll(output, 0o755); err != nil {
				return fmt.Errorf("create output directory: %w", err)
			}
			for name, content := range files {
				if err := os.WriteFile(filepath.Join(output, name), content, 0o644); err != nil {
					return fmt.Errorf("write %s: %w", name, err)
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "generated %d file(s) into %s\n", len(files), output)
			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "Output directory for generated Go files (required)")
	cmd.Flags().StringVar(&pkg, "package", "cligen", "Go package name for generated files")
	_ = cmd.MarkFlagRequired("output")

	return cmd
}
