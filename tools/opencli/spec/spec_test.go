package spec

import (
	"encoding/json"
	"testing"
)

// TestNoNorwayProblem guards against the YAML 1.1 behaviour where scalars like
// n, y, no, yes, on, off are coerced to booleans. CLI specs routinely use these
// as single-letter flag shorts and aliases, so parsing must keep them strings
// (YAML 1.2 core schema). Only true/false may become booleans.
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

	out, err := ToJSON(in)
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	for _, k := range []string{"n", "y", "no", "yes", "on", "off", "yPlain", "nPlain", "quoted"} {
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

// TestParse checks decoding into the IR, including the $ref union and the
// argument required-default (nil means true).
func TestParse(t *testing.T) {
	in := []byte(`
opencli: "1.0.0"
info: {title: demo, version: "1.0.0"}
commands:
  - name: get
    operationId: thingGet
    arguments:
      - name: ID
      - name: REST
        required: false
        variadic: true
    flags:
      - $ref: "#/components/flags/output"
components:
  flags:
    output: {name: output, short: o}
`)

	s, err := Parse(in)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(s.Commands) != 1 {
		t.Fatalf("got %d commands, want 1", len(s.Commands))
	}
	c := s.Commands[0]
	if c.OperationID != "thingGet" {
		t.Errorf("operationId: got %q", c.OperationID)
	}
	if !c.Arguments[0].IsRequired() {
		t.Error("ID should default to required")
	}
	if c.Arguments[1].IsRequired() {
		t.Error("REST is required:false, should not be required")
	}
	if !c.Flags[0].IsRef() || c.Flags[0].Ref != "#/components/flags/output" {
		t.Errorf("flag 0 should be a $ref, got %+v", c.Flags[0])
	}
	if s.Components == nil || s.Components.Flags["output"].Short != "o" {
		t.Error("component flag not decoded")
	}
}
