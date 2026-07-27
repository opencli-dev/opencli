package codegen

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Southclaws/schemancer/schemancer"
	"github.com/Southclaws/schemancer/schemancer/generators"
	"github.com/Southclaws/schemancer/schemancer/generators/golang"
	"github.com/Southclaws/schemancer/schemancer/ir"
	"github.com/google/jsonschema-go/jsonschema"

	"github.com/opencli-dev/opencli/tools/opencli/spec"
)

const componentsSchemaRefPrefix = "#/components/schemas/"

// buildSchemaDocument reshapes an OpenCLI spec's components.schemas map into
// a single JSON Schema document schemancer can consume: schemancer discovers
// named types from a document's $defs, not a nested components.schemas map,
// so every schema is rehosted under $defs and every $ref that pointed at
// #/components/schemas/X is rewritten to #/$defs/X.
func buildSchemaDocument(schemas map[string]json.RawMessage) (*jsonschema.Schema, error) {
	defs := make(map[string]any, len(schemas))
	for name, raw := range schemas {
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("components.schemas.%s: %w", name, err)
		}
		defs[name] = rewriteComponentRefs(value)
	}

	data, err := json.Marshal(map[string]any{"$defs": defs})
	if err != nil {
		return nil, err
	}

	var doc jsonschema.Schema
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("assembling schema document: %w", err)
	}

	return &doc, nil
}

// rewriteComponentRefs walks a decoded JSON value, rewriting every "$ref"
// string that points into #/components/schemas/ to point into #/$defs/
// instead, leaving every other value untouched.
func rewriteComponentRefs(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			if key == "$ref" {
				if ref, ok := child.(string); ok && strings.HasPrefix(ref, componentsSchemaRefPrefix) {
					out[key] = "#/$defs/" + strings.TrimPrefix(ref, componentsSchemaRefPrefix)
					continue
				}
			}
			out[key] = rewriteComponentRefs(child)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = rewriteComponentRefs(child)
		}
		return out
	default:
		return value
	}
}

// generateSchemaTypes turns an OpenCLI spec's components.schemas into Go
// source declaring one type per schema, via schemancer.
func generateSchemaTypes(schemas map[string]json.RawMessage, pkg string) ([]byte, error) {
	doc, err := buildSchemaDocument(schemas)
	if err != nil {
		return nil, err
	}

	opts := generators.GlobalOptions{
		Language: generators.LanguageGo,
		// schemancer's default "uri" mapping (net/url.URL) has no JSON
		// (un)marshaler, so it round-trips as a struct of URL parts instead
		// of the plain string the schema declares — plain string is the only
		// mapping that preserves the documented wire shape.
		FormatTypeMapping: map[ir.IRFormat]generators.FormatTypeMapping{
			ir.IRFormatURI: {Type: "string"},
		},
	}
	files, err := schemancer.Generate(doc, opts, golang.WithPackageName(pkg))
	if err != nil {
		return nil, fmt.Errorf("schemancer: %w", err)
	}
	if len(files) != 1 {
		return nil, fmt.Errorf("schemancer: expected 1 generated file, got %d", len(files))
	}

	return files[0].Content, nil
}

// resolveOutputType finds the Go type name a command's output should be
// returned as, by resolving every schema'd OutputFormat's $ref to its
// component schema name. Returns "" if the command declares no output
// schema. Errors if a command's schema'd formats disagree on the type.
func resolveOutputType(out *spec.Output) (string, error) {
	if out == nil {
		return "", nil
	}

	name := ""
	for _, f := range out.Formats {
		// json and yaml are the only encodings codegen can auto-serialize a
		// result with; any other format (jsonl's per-item stream, or a
		// human/plain format, or some other named encoding) stays the
		// handler's own responsibility rather than an auto-encoded type.
		if len(f.Schema) == 0 || (f.Format != "json" && f.Format != "yaml") {
			continue
		}

		ref, err := schemaRef(f.Schema)
		if err != nil {
			return "", fmt.Errorf("output format %q: %w", f.Format, err)
		}
		if ref == "" {
			// An inline schema with no $ref (e.g. an ad hoc object shape not
			// meant to be a shared, named type) has no Go type to generate,
			// so it stays out of scope for the typed-return feature — the
			// handler keeps writing that format's output itself, same as a
			// command with no output schema at all.
			continue
		}

		if name == "" {
			name = ref
		} else if name != ref {
			return "", fmt.Errorf("output formats %q and %q resolve to different schemas (%s vs %s); a command can only return one output type", out.Formats[0].Format, f.Format, name, ref)
		}
	}

	return name, nil
}

// schemaRef resolves an output format's schema to a Go type name: either a
// direct $ref to a components.schemas entry, or a bare "type: array, items:
// $ref" wrapper around one (e.g. a command with no pagination that returns
// every result as a plain array), which resolves to a slice of that type.
func schemaRef(raw json.RawMessage) (string, error) {
	var wrapper struct {
		Ref   string `json:"$ref"`
		Type  string `json:"type"`
		Items struct {
			Ref string `json:"$ref"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return "", err
	}

	if wrapper.Ref != "" {
		return refTypeName(wrapper.Ref), nil
	}
	if wrapper.Type == "array" && wrapper.Items.Ref != "" {
		return "[]" + refTypeName(wrapper.Items.Ref), nil
	}

	return "", nil
}

func refTypeName(ref string) string {
	idx := strings.LastIndex(ref, "/")
	return ref[idx+1:]
}
