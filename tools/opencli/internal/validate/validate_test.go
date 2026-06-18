package validate

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestNoNorwayProblem guards against the YAML 1.1 behaviour where scalars like
// n, y, no, yes, on, off are coerced to booleans. CLI specs routinely use these
// as single-letter flag shorts and aliases, so the parser must keep them as
// strings (YAML 1.2 core schema). Only true/false may become booleans.
func TestNoNorwayProblem(t *testing.T) {
	in := []byte(`
n: n
y: y
no: no
yes: yes
on: on
off: off
yPlain: Y
nPlain: N
quoted: "true"
realTrue: true
realFalse: false
`)

	out, err := yamlToJSON(in)
	if err != nil {
		t.Fatalf("yamlToJSON: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	wantString := []string{"n", "y", "no", "yes", "on", "off", "yPlain", "nPlain", "quoted"}
	for _, k := range wantString {
		if got, ok := m[k].(string); !ok {
			t.Errorf("key %q: got %T (%v), want string", k, m[k], m[k])
		} else if got == "" {
			t.Errorf("key %q: unexpectedly empty", k)
		}
	}

	if v, ok := m["realTrue"].(bool); !ok || v != true {
		t.Errorf("realTrue: got %T (%v), want bool true", m["realTrue"], m["realTrue"])
	}
	if v, ok := m["realFalse"].(bool); !ok || v != false {
		t.Errorf("realFalse: got %T (%v), want bool false", m["realFalse"], m["realFalse"])
	}
}

// TestSingleLetterShortValidates is an end-to-end check that a flag with a
// short of "n" survives parsing and validates against the OpenCLI schema —
// i.e. it stays a one-character string rather than becoming a boolean.
func TestSingleLetterShortValidates(t *testing.T) {
	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	spec := []byte(`
opencli: "1.0.0"
info:
  title: demo
  version: "1.0.0"
commands:
  - name: remove
    aliases: [n]
    flags:
      - name: dry-run
        short: n
        type: boolean
`)

	if err := s.Validate(spec); err != nil {
		t.Fatalf("expected valid spec, got: %v", err)
	}
}

// TestOneOfInEmbeddedSchema confirms that specs may use full JSON Schema
// vocabulary (oneOf, anyOf, ...) inside components.schemas / output schemas,
// since those fields reference the 2020-12 meta-schema.
func TestOneOfInEmbeddedSchema(t *testing.T) {
	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	spec := []byte(`
opencli: "1.0.0"
info: {title: demo, version: "1.0.0"}
commands:
  - name: get
    output:
      formats:
        - format: json
          schema:
            $ref: "#/components/schemas/Result"
components:
  schemas:
    Result:
      oneOf:
        - type: object
          properties:
            ok: {type: boolean}
        - type: string
`)

	if err := s.Validate(spec); err != nil {
		t.Fatalf("oneOf in embedded schema should be valid, got: %v", err)
	}
}

// TestInvalidEmbeddedSchemaRejected confirms embedded schemas are themselves
// validated against the meta-schema (a non-schema 'type' value is caught).
func TestInvalidEmbeddedSchemaRejected(t *testing.T) {
	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	spec := []byte(`
opencli: "1.0.0"
info: {title: demo, version: "1.0.0"}
commands:
  - name: get
components:
  schemas:
    Bad:
      type: 123
`)

	err = s.Validate(spec)
	if err == nil {
		t.Fatal("expected invalid embedded schema to be rejected")
	}
	if got := Issues(err); len(got) == 0 {
		t.Fatalf("expected at least one issue, got none")
	}
}

// TestIssuesAreLeafLevel ensures the formatter surfaces specific leaf
// violations rather than generic structural "validation failed" messages.
func TestIssuesAreLeafLevel(t *testing.T) {
	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	spec := []byte(`
opencli: "1.0.0"
info: {title: x, version: "1"}
commands:
  - name: bad
    flags:
      - {name: foo, short: xx}
`)

	issues := Issues(s.Validate(spec))
	if len(issues) == 0 {
		t.Fatal("expected issues, got none")
	}
	for _, iss := range issues {
		if strings.Contains(iss.Message, "validation failed") {
			t.Errorf("issue should be a concrete leaf, got generic: %q", iss)
		}
		if !strings.HasPrefix(iss.Location, "/commands/0/flags/0/short") {
			t.Errorf("unexpected issue location: %q", iss)
		}
	}
}
