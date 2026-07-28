package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/opencli-dev/opencli/tools/opencli/generator"
	_ "github.com/opencli-dev/opencli/tools/opencli/generators/go_cobra"
	_ "github.com/opencli-dev/opencli/tools/opencli/generators/rust_clap"
	opencliir "github.com/opencli-dev/opencli/tools/opencli/ir"
	"github.com/opencli-dev/opencli/tools/opencli/validate"
)

func newGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate command scaffolding from an OpenCLI specification",
		Long: "Each <language>-<framework> generator has its own command and options.\n" +
			"The specification is validated and converted to a shared IR before the\n" +
			"selected generator runs.",
		Args: cobra.NoArgs,
	}
	for _, tag := range generator.Tags() {
		gen, _ := generator.Lookup(tag)
		cmd.AddCommand(newGeneratorCmd(tag, gen))
	}
	return cmd
}

func newGeneratorCmd(tag string, gen generator.Generator) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   tag + " [spec]",
		Short: gen.Short(),
		Long:  gen.Long() + "\n\nPass '-' or omit the specification path to read from stdin.",
		Args:  cobra.MaximumNArgs(1),
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

			model, err := opencliir.Build(s)
			if err != nil {
				return fmt.Errorf("build generator IR: %w", err)
			}

			files, err := gen.Generate(model)
			if err != nil {
				return fmt.Errorf("generate: %w", err)
			}

			if err := os.MkdirAll(output, 0o755); err != nil {
				return fmt.Errorf("create output directory: %w", err)
			}
			for _, file := range files {
				destination, err := generatedFilePath(output, file.Path)
				if err != nil {
					return err
				}
				if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
					return fmt.Errorf("create output directory for %s: %w", file.Path, err)
				}
				if err := os.WriteFile(destination, file.Content, 0o644); err != nil {
					return fmt.Errorf("write %s: %w", file.Path, err)
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "generated %d file(s) into %s\n", len(files), output)
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output directory for generated files (required)")
	gen.ConfigureFlags(cmd.Flags())
	_ = cmd.MarkFlagRequired("output")
	return cmd
}

func generatedFilePath(output, generatedPath string) (string, error) {
	clean := filepath.Clean(generatedPath)
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("generator returned invalid output path %q", generatedPath)
	}
	return filepath.Join(output, clean), nil
}
