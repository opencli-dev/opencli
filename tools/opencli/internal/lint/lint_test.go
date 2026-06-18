package lint

import (
	"strings"
	"testing"

	"github.com/opencli-dev/opencli/tools/opencli/spec"
)

// issues parses a YAML spec and returns the lint messages.
func issues(t *testing.T, doc string) []string {
	t.Helper()
	s, err := spec.Parse([]byte(doc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var out []string
	for _, iss := range Check(s) {
		out = append(out, iss.String())
	}
	return out
}

// hasMatch reports whether any message contains substr.
func hasMatch(msgs []string, substr string) bool {
	for _, m := range msgs {
		if strings.Contains(m, substr) {
			return true
		}
	}
	return false
}

func TestCleanSpecHasNoIssues(t *testing.T) {
	doc := `
opencli: "1.0.0"
info: {title: demo, version: "1.0.0"}
commands:
  - name: get
    operationId: thingGet
    arguments:
      - {name: ID, required: true}
      - {name: REST, required: false, variadic: true}
    flags:
      - {name: output, short: o, choices: [json]}
    output:
      formatFlag: output
      formats: [{format: json}]
`
	if msgs := issues(t, doc); msgs != nil {
		t.Errorf("expected no issues, got: %v", msgs)
	}
}

func TestDanglingRef(t *testing.T) {
	doc := `
opencli: "1.0.0"
commands:
  - name: go
    flags:
      - $ref: "#/components/flags/missing"
components:
  flags:
    present: {name: present}
`
	if !hasMatch(issues(t, doc), "does not resolve") {
		t.Error("expected dangling-ref issue")
	}
}

func TestDuplicateOperationID(t *testing.T) {
	doc := `
opencli: "1.0.0"
commands:
  - {name: a, operationId: dup}
  - {name: b, operationId: dup}
`
	if !hasMatch(issues(t, doc), "duplicate operationId") {
		t.Error("expected duplicate operationId issue")
	}
}

func TestSiblingNameCollision(t *testing.T) {
	doc := `
opencli: "1.0.0"
commands:
  - {name: list}
  - {name: get, aliases: [list]}
`
	if !hasMatch(issues(t, doc), "collides with a sibling") {
		t.Error("expected sibling collision issue")
	}
}

func TestDuplicateFlag(t *testing.T) {
	doc := `
opencli: "1.0.0"
commands:
  - name: go
    flags:
      - {name: out, short: o}
      - {name: other, short: o}
`
	if !hasMatch(issues(t, doc), "duplicate short flag") {
		t.Error("expected duplicate short flag issue")
	}
}

func TestArgumentOrdering(t *testing.T) {
	doc := `
opencli: "1.0.0"
commands:
  - name: go
    arguments:
      - {name: OPT, required: false}
      - {name: REQ, required: true}
`
	if !hasMatch(issues(t, doc), "follows optional argument") {
		t.Error("expected required-after-optional issue")
	}
}

func TestVariadicNotLast(t *testing.T) {
	doc := `
opencli: "1.0.0"
commands:
  - name: go
    arguments:
      - {name: REST, variadic: true}
      - {name: TAIL}
`
	if !hasMatch(issues(t, doc), "must be the last argument") {
		t.Error("expected variadic-not-last issue")
	}
}

func TestCountAndRepeatable(t *testing.T) {
	doc := `
opencli: "1.0.0"
commands:
  - name: go
    flags:
      - {name: v, count: true, repeatable: true}
`
	if !hasMatch(issues(t, doc), "cannot be combined") {
		t.Error("expected count+repeatable issue")
	}
}

func TestFlagGroupUnknownFlag(t *testing.T) {
	doc := `
opencli: "1.0.0"
commands:
  - name: go
    flags:
      - {name: a, type: boolean}
    flagGroups:
      - {type: mutuallyExclusive, flags: [a, ghost]}
`
	if !hasMatch(issues(t, doc), `unknown flag "ghost"`) {
		t.Error("expected flag-group unknown-flag issue")
	}
}

func TestFormatFlagUnknown(t *testing.T) {
	doc := `
opencli: "1.0.0"
commands:
  - name: go
    output:
      formatFlag: nope
      formats: [{format: json}]
`
	if !hasMatch(issues(t, doc), `formatFlag references unknown flag "nope"`) {
		t.Error("expected formatFlag unknown issue")
	}
}

// TestGlobalFlagSatisfiesGroup confirms inherited global flags count toward a
// command's available flags for groups and formatFlag.
func TestGlobalFlagSatisfiesGroup(t *testing.T) {
	doc := `
opencli: "1.0.0"
flags:
  - {name: output, short: o}
commands:
  - name: go
    output:
      formatFlag: output
      formats: [{format: json}]
`
	if msgs := issues(t, doc); msgs != nil {
		t.Errorf("global flag should satisfy formatFlag, got: %v", msgs)
	}
}
