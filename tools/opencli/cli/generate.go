package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/opencli-dev/opencli/tools/opencli/generator"
	"github.com/opencli-dev/opencli/tools/opencli/generators/go_cobra"
	"github.com/opencli-dev/opencli/tools/opencli/generators/rust_clap"
	opencliir "github.com/opencli-dev/opencli/tools/opencli/ir"
	"github.com/opencli-dev/opencli/tools/opencli/validate"
)

func generateGoCobraHandler(_ context.Context, _ *cobra.Command, commandIO IO, params GenerateGoCobraParams) error {
	return generate(commandIO, params.Spec, params.Output, go_cobra.New(params.Package))
}

func generateRustClapHandler(_ context.Context, _ *cobra.Command, commandIO IO, params GenerateRustClapParams) error {
	return generate(commandIO, params.Spec, params.Output, rust_clap.New(params.Module))
}

func generate(commandIO IO, path, output string, gen generator.Generator) error {
	if path == "" {
		path = "-"
	}
	data, src, err := readInput(path, commandIO.In)
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
		fmt.Fprintf(commandIO.Out, "✗ %s is not a valid OpenCLI specification (%d issue(s))\n\n", src, len(verr.Issues))
		for _, iss := range verr.Issues {
			fmt.Fprintf(commandIO.Out, "  %s\n", iss)
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

	fmt.Fprintf(commandIO.Out, "generated %d file(s) into %s\n", len(files), output)
	return nil
}

func generatedFilePath(output, generatedPath string) (string, error) {
	clean := filepath.Clean(generatedPath)
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("generator returned invalid output path %q", generatedPath)
	}
	return filepath.Join(output, clean), nil
}
