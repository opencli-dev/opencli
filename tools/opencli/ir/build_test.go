package ir

import (
	"encoding/json"
	"testing"

	"github.com/opencli-dev/opencli/tools/opencli/spec"
)

func TestBuildResolvesComponentsAndBuildsSchemaIR(t *testing.T) {
	input := &spec.Spec{
		Commands: []spec.Command{{Ref: "#/components/commands/get"}},
		Components: &spec.Components{
			Commands: map[string]spec.Command{
				"get": {Name: "get", OperationID: "get", Flags: []spec.Flag{{Ref: "#/components/flags/format"}}},
			},
			Flags: map[string]spec.Flag{
				"format": {Name: "format", Choices: []string{"json"}},
			},
			Schemas: map[string]json.RawMessage{
				"Result": json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}}}`),
			},
		},
	}

	result, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Commands[0].Name != "get" || result.Commands[0].Flags[0].Name != "format" {
		t.Fatalf("references were not resolved: %#v", result.Commands[0])
	}
	if result.Schema == nil || len(result.Schema.Types) != 1 || result.Schema.Types[0].Name != "Result" {
		t.Fatalf("schema IR was not built: %#v", result.Schema)
	}
}
