package cli

import (
	"context"
	"github.com/spf13/cobra"
)

type GenerateGoCobraParams struct {
	Output  string
	Package string
	Spec    string
}
type GenerateGoCobraHandler func(ctx context.Context, cmd *cobra.Command, io IO, p GenerateGoCobraParams) error

func newGenerateGoCobraCommand(generateGoCobra GenerateGoCobraHandler) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "go-cobra [spec]",
		Short: "Generate Go command scaffolding using Cobra.",
		Long: `Generate typed Go handlers, parameter structs, Cobra constructors,
schema types, and a root command assembler.

Pass - or omit the specification path to read from stdin.
`,
		Args: rangeArgs(0, 1),
	}

	var rawOutput string
	cmd.Flags().StringVarP(&rawOutput, "output", "o", "", "Output directory for generated files.")
	var rawPackage string
	cmd.Flags().StringVar(&rawPackage, "package", "cligen", "Go package name for generated files.")
	_ = cmd.MarkFlagRequired("output")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		var rawSpec string
		if len(args) > 0 {
			rawSpec = args[0]
		}

		p := GenerateGoCobraParams{
			Output:  rawOutput,
			Package: rawPackage,
			Spec:    rawSpec,
		}

		return generateGoCobra(cmd.Context(), cmd, newIO(cmd), p)
	}

	return cmd
}

type GenerateRustClapParams struct {
	Output string
	Module string
	Spec   string
}
type GenerateRustClapHandler func(ctx context.Context, cmd *cobra.Command, io IO, p GenerateRustClapParams) error

func newGenerateRustClapCommand(generateRustClap GenerateRustClapHandler) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rust-clap [spec]",
		Short: "Generate Rust command scaffolding using Clap.",
		Long: `Generate a typed Rust module using Clap's derive API, including nested
subcommands, value enums, argument relationships, and schema types.

Pass - or omit the specification path to read from stdin.
`,
		Args: rangeArgs(0, 1),
	}

	var rawOutput string
	cmd.Flags().StringVarP(&rawOutput, "output", "o", "", "Output directory for generated files.")
	var rawModule string
	cmd.Flags().StringVar(&rawModule, "module", "opencli_gen", "Rust module and output filename without the .rs suffix.")
	_ = cmd.MarkFlagRequired("output")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		var rawSpec string
		if len(args) > 0 {
			rawSpec = args[0]
		}

		p := GenerateRustClapParams{
			Output: rawOutput,
			Module: rawModule,
			Spec:   rawSpec,
		}

		return generateRustClap(cmd.Context(), cmd, newIO(cmd), p)
	}

	return cmd
}
func NewGenerateCommand(
	generateGoCobra GenerateGoCobraHandler,
	generateRustClap GenerateRustClapHandler,
) GenerateCommand {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate command scaffolding from an OpenCLI specification.",
		Long: `Each language-framework generator has its own command and options. The
specification is validated and converted to a shared IR before the
selected generator runs.
`,
	}
	cmd.AddCommand(
		newGenerateGoCobraCommand(generateGoCobra),
		newGenerateRustClapCommand(generateRustClap),
	)
	return GenerateCommand(cmd)
}

type GenerateCommand *cobra.Command
