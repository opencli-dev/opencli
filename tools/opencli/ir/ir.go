// Package ir defines the normalized, language-neutral representation passed
// from OpenCLI specifications to source generators.
package ir

import (
	schemair "github.com/Southclaws/schemancer/schemancer/ir"

	"github.com/opencli-dev/opencli/tools/opencli/spec"
)

// IR is the only input a source generator receives. Component references are
// expanded and component schemas are converted to Schemancer's generic type IR
// before this value is returned by Build.
type IR struct {
	Info     *Info
	Flags    []Flag
	Commands []Command
	Schema   *schemair.IR
}

// The OpenCLI vocabulary below remains shared with the parsed specification,
// while Build gives it IR semantics by resolving every reference first.
type Info = spec.Info
type Flag = spec.Flag
type Argument = spec.Argument
type Command = spec.Command
type Example = spec.Example
type FlagGroup = spec.FlagGroup
type Output = spec.Output
type OutputFormat = spec.OutputFormat
