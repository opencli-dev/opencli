// Package validate is the OpenCLI validation pipeline. It runs structural
// validation (against the embedded JSON Schema) followed by semantic checks
// over the decoded IR, returning the parsed spec and, on failure, an *Error
// enumerating the specific problems.
package validate

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/opencli-dev/opencli/tools/opencli/lint"
	"github.com/opencli-dev/opencli/tools/opencli/schema"
	"github.com/opencli-dev/opencli/tools/opencli/spec"
)

// fallbackID is used to register the schema when it declares no $id.
const fallbackID = "mem://opencli/schema.json"

// Validator holds the compiled OpenCLI schema and validates documents against
// it. It is reusable and safe to validate many documents.
type Validator struct {
	compiled *jsonschema.Schema
}

// Error reports that a document is not a valid OpenCLI spec, carrying the
// specific structural and semantic issues that were found.
type Error struct {
	Issues []spec.Issue
}

func (e *Error) Error() string {
	switch len(e.Issues) {
	case 0:
		return "invalid OpenCLI spec"
	case 1:
		return "invalid OpenCLI spec: " + e.Issues[0].String()
	default:
		return fmt.Sprintf("invalid OpenCLI spec: %d issues", len(e.Issues))
	}
}

// New compiles the embedded OpenCLI schema.
func New() (*Validator, error) {
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
	// `format` is annotation-only by default in 2020-12; assert it so the
	// schema's own uri/email fields and authors' embedded-schema formats are
	// enforced rather than silently ignored.
	c.AssertFormat()
	if err := c.AddResource(id, doc); err != nil {
		return nil, fmt.Errorf("register embedded schema: %w", err)
	}
	compiled, err := c.Compile(id)
	if err != nil {
		return nil, fmt.Errorf("compile embedded schema: %w", err)
	}

	return &Validator{compiled: compiled}, nil
}

// Check validates a document (JSON or YAML) and returns the decoded spec.
//
// Step 1 is structural validation against the schema; step 2 is semantic
// checks over the IR, run only when the document is structurally valid (there
// is no point linting a malformed document). On any validation problem it
// returns the best-effort decoded spec together with an *Error listing the
// issues. A plain error is returned only for input that cannot be decoded.
func (v *Validator) Check(data []byte) (*spec.Spec, error) {
	jsonBytes, err := spec.ToJSON(data)
	if err != nil {
		return nil, fmt.Errorf("parse spec: %w", err)
	}

	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("decode spec: %w", err)
	}

	var issues []spec.Issue
	if verr := v.compiled.Validate(inst); verr != nil {
		var ve *jsonschema.ValidationError
		if errors.As(verr, &ve) {
			issues = append(issues, structuralIssues(ve)...)
		} else {
			return nil, verr
		}
	}

	s, perr := spec.Parse(data)
	if perr != nil {
		// Should not happen once ToJSON succeeded, but stay defensive.
		return nil, fmt.Errorf("decode spec: %w", perr)
	}

	// Only run semantic checks on a structurally valid document.
	if len(issues) == 0 {
		issues = append(issues, lint.Check(s)...)
	}

	if len(issues) > 0 {
		return s, &Error{Issues: issues}
	}
	return s, nil
}

// msgPrinter localises error-kind messages. English is the only catalogue.
var msgPrinter = message.NewPrinter(language.English)

// structuralIssues flattens a schema ValidationError to its leaf causes, so
// only specific violations surface — the structural branch nodes (if/then/else,
// $ref, properties) that merely say "validation failed" are skipped.
func structuralIssues(ve *jsonschema.ValidationError) []spec.Issue {
	var out []spec.Issue
	seen := map[string]bool{}

	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) > 0 {
			for _, c := range e.Causes {
				walk(c)
			}
			return
		}
		iss := spec.Issue{
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
