// Package generator defines the language- and framework-neutral code generator
// boundary used by the OpenCLI command.
package generator

import (
	"sort"

	"github.com/spf13/pflag"

	"github.com/opencli-dev/opencli/tools/opencli/ir"
)

// GeneratedFile is one generated artifact. Path is relative to the selected
// output directory and may contain subdirectories.
type GeneratedFile struct {
	Path    string
	Content []byte
}

// Generator produces source files for one language/framework pairing.
type Generator interface {
	Short() string
	Long() string
	ConfigureFlags(flags *pflag.FlagSet)
	Generate(input *ir.IR) ([]GeneratedFile, error)
}

type Factory func() Generator

var registry = map[string]Factory{}

// Register makes a generator available under a <language>-<framework> tag.
func Register(tag string, factory Factory) {
	if tag == "" || factory == nil {
		panic("generator: tag and implementation are required")
	}
	if _, exists := registry[tag]; exists {
		panic("generator: duplicate registration for " + tag)
	}
	registry[tag] = factory
}

// Lookup returns the generator registered for tag.
func Lookup(tag string) (Generator, bool) {
	factory, ok := registry[tag]
	if !ok {
		return nil, false
	}
	return factory(), true
}

// Tags returns all registered language/framework tags in stable order.
func Tags() []string {
	tags := make([]string, 0, len(registry))
	for tag := range registry {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}
