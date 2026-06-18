// Package validate checks OpenCLI specification documents against the embedded
// OpenCLI JSON Schema.
package validate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/opencli-dev/opencli/tools/opencli/internal/schema"
)

// fallbackID is used to register the schema when it declares no $id.
const fallbackID = "mem://opencli/schema.json"

// Schema is a compiled OpenCLI schema ready to validate documents.
type Schema struct {
	compiled *jsonschema.Schema
}

// Load compiles the embedded OpenCLI schema. The compiled schema is reusable
// and safe to validate many documents against.
func Load() (*Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema.OpenCLI))
	if err != nil {
		return nil, fmt.Errorf("parse embedded schema: %w", err)
	}

	// Compile against the schema's own $id so the tool keeps working if the
	// canonical $id changes; fall back to a synthetic URL if it has none.
	id := fallbackID
	if obj, ok := doc.(map[string]any); ok {
		if v, ok := obj["$id"].(string); ok && v != "" {
			id = v
		}
	}

	c := jsonschema.NewCompiler()
	if err := c.AddResource(id, doc); err != nil {
		return nil, fmt.Errorf("register embedded schema: %w", err)
	}

	compiled, err := c.Compile(id)
	if err != nil {
		return nil, fmt.Errorf("compile embedded schema: %w", err)
	}

	return &Schema{compiled: compiled}, nil
}

// Validate parses a spec document (JSON or YAML) and validates it against the
// OpenCLI schema. A nil error means the document is a valid OpenCLI spec; a
// *jsonschema.ValidationError describes why it is not.
func (s *Schema) Validate(spec []byte) error {
	jsonBytes, err := yamlToJSON(spec)
	if err != nil {
		return fmt.Errorf("parse spec: %w", err)
	}

	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(jsonBytes))
	if err != nil {
		return fmt.Errorf("decode spec: %w", err)
	}

	return s.compiled.Validate(inst)
}

// Issue is a single, human-readable validation problem located by a JSON
// Pointer into the instance document.
type Issue struct {
	Location string // e.g. "/commands/0/flags/0/short"; "" means the root
	Message  string
}

// String renders the issue as "location: message".
func (i Issue) String() string {
	loc := i.Location
	if loc == "" {
		loc = "(root)"
	}
	return loc + ": " + i.Message
}

// msgPrinter localises error-kind messages. English is the only catalogue.
var msgPrinter = message.NewPrinter(language.English)

// Issues flattens a validation error into a deduplicated, ordered list of
// concrete problems. It walks to the leaves of the error tree so only the
// specific violations surface — the structural branch nodes (if/then/else,
// $ref, properties) that just say "validation failed" are skipped. It returns
// nil for non-schema errors (e.g. parse or I/O failures).
func Issues(err error) []Issue {
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return nil
	}

	var out []Issue
	seen := map[string]bool{}

	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) > 0 {
			for _, c := range e.Causes {
				walk(c)
			}
			return
		}
		iss := Issue{
			Location: jsonPointer(e.InstanceLocation),
			Message:  e.ErrorKind.LocalizedString(msgPrinter),
		}
		if key := iss.String(); !seen[key] {
			seen[key] = true
			out = append(out, iss)
		}
	}
	walk(ve)

	return out
}

// jsonPointer renders a location path as an RFC 6901 JSON Pointer.
func jsonPointer(loc []string) string {
	if len(loc) == 0 {
		return ""
	}
	var b strings.Builder
	for _, tok := range loc {
		b.WriteByte('/')
		tok = strings.ReplaceAll(tok, "~", "~0")
		tok = strings.ReplaceAll(tok, "/", "~1")
		b.WriteString(tok)
	}
	return b.String()
}

// yamlToJSON converts a YAML or JSON document to JSON bytes using a YAML 1.2
// parser. This keeps single-character scalars like `n` and `y` as strings
// rather than coercing them to booleans (the YAML 1.1 "Norway problem"), which
// matters for single-letter flag shorts and command aliases. JSON input parses
// unchanged, since JSON is a subset of YAML.
func yamlToJSON(b []byte) ([]byte, error) {
	var v any
	if err := yaml.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}
