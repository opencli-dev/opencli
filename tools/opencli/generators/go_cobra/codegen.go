// Package go_cobra implements the go-cobra generator. It turns a validated
// OpenCLI IR into Go source: one
// file per top-level command (a Handler func type, a Params struct, and a
// constructor building the parsed *cobra.Command), plus a shared file and a
// root command assembler. It never emits business logic — every generated
// constructor's RunE only parses and validates the command line, then hands
// a typed Params value to a caller-supplied Handler.
package go_cobra

import (
	"bytes"
	"fmt"
	"go/format"
	"sort"
	"strings"

	"github.com/spf13/pflag"

	genapi "github.com/opencli-dev/opencli/tools/opencli/generator"
	opencliir "github.com/opencli-dev/opencli/tools/opencli/ir"
)

const tag = "go-cobra"

type cobraGenerator struct{ packageName string }

func init() {
	genapi.Register(tag, func() genapi.Generator { return &cobraGenerator{packageName: "cligen"} })
}

func (*cobraGenerator) Short() string { return "Generate Go command scaffolding using Cobra" }

func (*cobraGenerator) Long() string {
	return "Generate typed Go handlers, parameter structs, Cobra constructors, schema types, and a root command assembler."
}

func (g *cobraGenerator) ConfigureFlags(flags *pflag.FlagSet) {
	flags.StringVar(&g.packageName, "package", "cligen", "Go package name for generated files")
}

// Generate renders a full set of Go files from normalized OpenCLI IR.
func (g *cobraGenerator) Generate(s *opencliir.IR) ([]genapi.GeneratedFile, error) {
	pkg := g.packageName
	globalFlags := make([]resolvedFlag, len(s.Flags))
	for i, f := range s.Flags {
		globalFlags[i] = classifyFlag(f, "Global")
	}

	renderer := &generator{globalFlags: globalFlags}

	var files []genapi.GeneratedFile
	var tops []topEntry

	for _, c := range s.Commands {
		var body strings.Builder
		// The returned nodeResult.CtorExpr is only meaningful to a parent
		// group node; top-level commands are wired externally (e.g. by fx),
		// not called from root, so it's discarded here.
		_, err := renderer.renderNode(&body, c, true, "")
		if err != nil {
			return nil, err
		}

		typeName := pascalCase(c.Name) + "Command"
		fmt.Fprintf(&body, "\ntype %s *cobra.Command\n", typeName)

		src, err := assembleFile(pkg, body.String())
		if err != nil {
			return nil, fmt.Errorf("command %q: %w", c.Name, err)
		}
		files = append(files, genapi.GeneratedFile{Path: fileName(c.Name), Content: src})

		tops = append(tops, topEntry{
			VarName:  camelCase(c.Name),
			TypeName: typeName,
		})
	}

	sharedBody, err := renderShared()
	if err != nil {
		return nil, err
	}
	shared, err := assembleFile(pkg, sharedBody)
	if err != nil {
		return nil, err
	}
	files = append(files, genapi.GeneratedFile{Path: "opencli.gen.go", Content: shared})

	if s.Schema != nil && len(s.Schema.Types) > 0 {
		schemas, err := generateSchemaTypes(s.Schema, pkg)
		if err != nil {
			return nil, err
		}
		files = append(files, genapi.GeneratedFile{Path: "schemas.gen.go", Content: schemas})
	}

	var root strings.Builder
	if err := renderRoot(&root, s, globalFlags, tops); err != nil {
		return nil, err
	}
	rootSource, err := assembleFile(pkg, root.String())
	if err != nil {
		return nil, err
	}
	files = append(files, genapi.GeneratedFile{Path: "root.gen.go", Content: rootSource})

	return files, nil
}

func fileName(commandName string) string {
	return strings.ReplaceAll(commandName, "-", "_") + ".gen.go"
}

// assembleFile prepends a package clause and an import block (computed by
// scanning body for the identifiers each optional import provides) to body,
// then formats the result with gofmt.
func assembleFile(pkg, body string) ([]byte, error) {
	var imports []string
	add := func(needle, path string) {
		if strings.Contains(body, needle) {
			imports = append(imports, path)
		}
	}
	add("context.", "context")
	add("fmt.", "fmt")
	add("json.", "encoding/json")
	add("yaml.", "github.com/goccy/go-yaml")
	add("os.", "os")
	add("time.", "time")
	add("uuid.", "github.com/google/uuid")
	add("cobra.", "github.com/spf13/cobra")
	add("io.Reader", "io")
	add("io.Writer", "io")
	sort.Strings(imports)

	var out bytes.Buffer
	fmt.Fprintf(&out, "package %s\n\n", pkg)
	if len(imports) > 0 {
		out.WriteString("import (\n")
		for _, imp := range imports {
			fmt.Fprintf(&out, "\t%s\n", goStringLiteral(imp))
		}
		out.WriteString(")\n")
	}
	out.WriteString(body)

	formatted, err := format.Source(out.Bytes())
	if err != nil {
		return nil, fmt.Errorf("gofmt: %w\n---\n%s", err, out.String())
	}
	return formatted, nil
}
