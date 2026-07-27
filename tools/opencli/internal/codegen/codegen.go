// Package codegen turns a validated OpenCLI spec.Spec into Go source: one
// file per top-level command (a Handler func type, a Params struct, and a
// constructor building the parsed *cobra.Command), plus a shared file and a
// root command assembler. It never emits business logic — every generated
// constructor's RunE only parses and validates the command line, then hands
// a typed Params value to a caller-supplied Handler.
package codegen

import (
	"bytes"
	"fmt"
	"go/format"
	"sort"
	"strings"

	"github.com/opencli-dev/opencli/tools/opencli/spec"
)

// Generate renders a full set of Go files for s into pkg. s is expected to
// have already passed validate.Check (structural + semantic); Generate
// re-resolves $refs itself but does not re-run lint checks, so a spec with
// dangling refs or ordering problems will produce incorrect or non-compiling
// output rather than a clean error.
func Generate(s *spec.Spec, pkg string) (map[string][]byte, error) {
	r := newResolver(s)

	rawGlobalFlags, err := r.flags(s.Flags)
	if err != nil {
		return nil, err
	}
	globalFlags := make([]resolvedFlag, len(rawGlobalFlags))
	for i, f := range rawGlobalFlags {
		globalFlags[i] = classifyFlag(f, "Global")
	}

	commands, err := r.commands(s.Commands)
	if err != nil {
		return nil, err
	}

	g := &generator{globalFlags: globalFlags}

	files := map[string][]byte{}
	var tops []topEntry

	for _, c := range commands {
		var body strings.Builder
		// The returned nodeResult.CtorExpr is only meaningful to a parent
		// group node; top-level commands are wired externally (e.g. by fx),
		// not called from root, so it's discarded here.
		_, err := g.renderNode(&body, c, true, "")
		if err != nil {
			return nil, err
		}

		typeName := pascalCase(c.Name) + "Command"
		fmt.Fprintf(&body, "\ntype %s *cobra.Command\n", typeName)

		src, err := assembleFile(pkg, body.String())
		if err != nil {
			return nil, fmt.Errorf("command %q: %w", c.Name, err)
		}
		files[fileName(c.Name)] = src

		tops = append(tops, topEntry{
			VarName:  camelCase(c.Name),
			TypeName: typeName,
		})
	}

	files["opencli.gen.go"], err = assembleFile(pkg, renderShared())
	if err != nil {
		return nil, err
	}

	if s.Components != nil && len(s.Components.Schemas) > 0 {
		files["schemas.gen.go"], err = generateSchemaTypes(s.Components.Schemas, pkg)
		if err != nil {
			return nil, err
		}
	}

	var root strings.Builder
	renderRoot(&root, s, globalFlags, tops)
	files["root.gen.go"], err = assembleFile(pkg, root.String())
	if err != nil {
		return nil, err
	}

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
