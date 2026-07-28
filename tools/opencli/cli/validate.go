package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/opencli-dev/opencli/tools/opencli/validate"
)

func validateHandler(_ context.Context, _ *cobra.Command, commandIO IO, params ValidateParams) error {
	path := params.Spec
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

	out := commandIO.Out
	if _, err := v.Check(data); err != nil {
		var verr *validate.Error
		if !errors.As(err, &verr) {
			return err
		}
		fmt.Fprintf(out, "✗ %s is not a valid OpenCLI specification (%d issue(s))\n\n", src, len(verr.Issues))
		for _, iss := range verr.Issues {
			fmt.Fprintf(out, "  %s\n", iss)
		}
		return ErrInvalid
	}

	fmt.Fprintf(out, "✓ %s is a valid OpenCLI specification\n", src)
	return nil
}

func readInput(path string, in io.Reader) (data []byte, source string, err error) {
	if path == "-" {
		data, err = io.ReadAll(in)
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
