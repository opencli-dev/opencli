package validate

import (
	"errors"
	"strings"
	"testing"
)

// check runs the pipeline and returns the issues (nil when valid). It fails the
// test on an unexpected non-validation error (e.g. undecodable input).
func check(t *testing.T, v *Validator, spec []byte) []string {
	t.Helper()
	_, err := v.Check(spec)
	if err == nil {
		return nil
	}
	var verr *Error
	if !errors.As(err, &verr) {
		t.Fatalf("unexpected error: %v", err)
	}
	msgs := make([]string, len(verr.Issues))
	for i, iss := range verr.Issues {
		msgs[i] = iss.String()
	}
	return msgs
}

func newV(t *testing.T) *Validator {
	t.Helper()
	v, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return v
}

func TestValidSpec(t *testing.T) {
	v := newV(t)
	spec := []byte(`
opencli: "1.0.0"
info: {title: demo, version: "1.0.0"}
commands:
  - name: remove
    aliases: [n]
    flags:
      - {name: dry-run, short: n, type: boolean}
`)
	if msgs := check(t, v, spec); msgs != nil {
		t.Errorf("expected valid, got issues: %v", msgs)
	}
}

func TestStructuralFormatAssertion(t *testing.T) {
	v := newV(t)
	bad := []byte(`
opencli: "1.0.0"
info: {title: x, version: "1.0.0", homepage: "not a url"}
commands: [{name: go}]
`)
	if check(t, v, bad) == nil {
		t.Error("expected malformed homepage uri to be rejected")
	}
}

func TestRemovedTypeAliases(t *testing.T) {
	v := newV(t)
	for _, removed := range []string{"url", "email", "duration"} {
		spec := []byte(`
opencli: "1.0.0"
info: {title: x, version: "1.0.0"}
commands: [{name: go, flags: [{name: f, type: ` + removed + `}]}]
`)
		if check(t, v, spec) == nil {
			t.Errorf("type: %s should be rejected (moved to format)", removed)
		}
	}
}

// TestEmbeddedSchemaIsRealJSONSchema locks in that components.schemas and
// output.formats[].schema are validated as genuine JSON Schema (Draft
// 2020-12) documents, not merely "is this an object". The compiler resolves
// https://json-schema.org/draft/2020-12/schema to jsonschema/v6's native
// Draft2020 implementation rather than fetching it, so this holds fully
// offline — a malformed embedded schema (bad `type` enum value, wrong-shaped
// `properties`) must surface as a structural issue against the meta-schema.
func TestEmbeddedSchemaIsRealJSONSchema(t *testing.T) {
	v := newV(t)
	spec := []byte(`
opencli: "1.0.0"
info: {title: x, version: "1.0.0"}
commands:
  - name: get
    operationId: thingGet
    output:
      formatFlag: format
      formats:
        - format: json
          schema:
            type: not-a-real-json-schema-type
            properties: "this should be an object, not a string"
    flags:
      - {name: format, choices: [json]}
`)
	msgs := check(t, v, spec)
	joined := strings.Join(msgs, "\n")
	if !strings.Contains(joined, "/schema/type") {
		t.Errorf("expected the bogus embedded `type` value to be rejected, got: %v", msgs)
	}
	if !strings.Contains(joined, "/schema/properties") {
		t.Errorf("expected the wrong-shaped embedded `properties` to be rejected, got: %v", msgs)
	}
}

func TestOneOfInEmbeddedSchema(t *testing.T) {
	v := newV(t)
	spec := []byte(`
opencli: "1.0.0"
info: {title: demo, version: "1.0.0"}
commands:
  - name: get
    output:
      formatFlag: format
      formats:
        - format: json
          schema:
            $ref: "#/components/schemas/Result"
    flags:
      - {name: format, choices: [json]}
components:
  schemas:
    Result:
      oneOf:
        - {type: object}
        - {type: string}
`)
	if msgs := check(t, v, spec); msgs != nil {
		t.Errorf("oneOf in embedded schema should be valid, got: %v", msgs)
	}
}

func TestStructuralIssuesAreLeafLevel(t *testing.T) {
	v := newV(t)
	spec := []byte(`
opencli: "1.0.0"
info: {title: x, version: "1.0.0"}
commands:
  - name: bad
    flags:
      - {name: foo, short: xx}
`)
	msgs := check(t, v, spec)
	if len(msgs) == 0 {
		t.Fatal("expected issues")
	}
	for _, m := range msgs {
		if strings.Contains(m, "validation failed") {
			t.Errorf("issue should be a concrete leaf, got generic: %q", m)
		}
	}
}

// TestSemanticRunsWhenStructurallyValid confirms step 2 runs and surfaces a
// dangling $ref that the schema alone cannot catch.
func TestSemanticRunsWhenStructurallyValid(t *testing.T) {
	v := newV(t)
	spec := []byte(`
opencli: "1.0.0"
info: {title: x, version: "1.0.0"}
commands:
  - name: go
    flags:
      - $ref: "#/components/flags/missing"
`)
	msgs := check(t, v, spec)
	if len(msgs) != 1 || !strings.Contains(msgs[0], "does not resolve") {
		t.Errorf("expected one dangling-ref issue, got: %v", msgs)
	}
}
