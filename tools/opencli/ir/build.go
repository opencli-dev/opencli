package ir

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Southclaws/schemancer/schemancer"
	"github.com/google/jsonschema-go/jsonschema"

	"github.com/opencli-dev/opencli/tools/opencli/spec"
)

// Build normalizes a validated OpenCLI specification into the representation
// shared by every source generator.
func Build(input *spec.Spec) (*IR, error) {
	r := resolver{components: input.Components}
	flags, err := r.flags(input.Flags)
	if err != nil {
		return nil, err
	}
	commands, err := r.commands(input.Commands)
	if err != nil {
		return nil, err
	}

	result := &IR{Info: input.Info, Flags: flags, Commands: commands}
	if input.Components != nil && len(input.Components.Schemas) > 0 {
		document, err := schemaDocument(input.Components.Schemas)
		if err != nil {
			return nil, err
		}
		result.Schema, err = schemancer.SchemaToIR(document)
		if err != nil {
			return nil, fmt.Errorf("components.schemas: %w", err)
		}
	}
	return result, nil
}

type resolver struct{ components *spec.Components }

func componentRef(ref string) (kind, name string, ok bool) {
	const prefix = "#/components/"
	if !strings.HasPrefix(ref, prefix) {
		return "", "", false
	}
	parts := strings.SplitN(strings.TrimPrefix(ref, prefix), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func (r resolver) flag(value spec.Flag) (spec.Flag, error) {
	if !value.IsRef() {
		return value, nil
	}
	kind, name, ok := componentRef(value.Ref)
	if !ok || kind != "flags" || r.components == nil {
		return spec.Flag{}, fmt.Errorf("dangling flag $ref %q", value.Ref)
	}
	resolved, ok := r.components.Flags[name]
	if !ok {
		return spec.Flag{}, fmt.Errorf("dangling flag $ref %q", value.Ref)
	}
	return resolved, nil
}

func (r resolver) argument(value spec.Argument) (spec.Argument, error) {
	if !value.IsRef() {
		return value, nil
	}
	kind, name, ok := componentRef(value.Ref)
	if !ok || kind != "arguments" || r.components == nil {
		return spec.Argument{}, fmt.Errorf("dangling argument $ref %q", value.Ref)
	}
	resolved, ok := r.components.Arguments[name]
	if !ok {
		return spec.Argument{}, fmt.Errorf("dangling argument $ref %q", value.Ref)
	}
	return resolved, nil
}

func (r resolver) command(value spec.Command) (spec.Command, error) {
	if !value.IsRef() {
		return value, nil
	}
	kind, name, ok := componentRef(value.Ref)
	if !ok || kind != "commands" || r.components == nil {
		return spec.Command{}, fmt.Errorf("dangling command $ref %q", value.Ref)
	}
	resolved, ok := r.components.Commands[name]
	if !ok {
		return spec.Command{}, fmt.Errorf("dangling command $ref %q", value.Ref)
	}
	return resolved, nil
}

func (r resolver) flags(values []spec.Flag) ([]spec.Flag, error) {
	result := make([]spec.Flag, 0, len(values))
	for _, value := range values {
		resolved, err := r.flag(value)
		if err != nil {
			return nil, err
		}
		result = append(result, resolved)
	}
	return result, nil
}

func (r resolver) arguments(values []spec.Argument) ([]spec.Argument, error) {
	result := make([]spec.Argument, 0, len(values))
	for _, value := range values {
		resolved, err := r.argument(value)
		if err != nil {
			return nil, err
		}
		result = append(result, resolved)
	}
	return result, nil
}

func (r resolver) commands(values []spec.Command) ([]spec.Command, error) {
	result := make([]spec.Command, 0, len(values))
	for _, value := range values {
		resolved, err := r.command(value)
		if err != nil {
			return nil, err
		}
		resolved.Flags, err = r.flags(resolved.Flags)
		if err != nil {
			return nil, err
		}
		resolved.Arguments, err = r.arguments(resolved.Arguments)
		if err != nil {
			return nil, err
		}
		resolved.Commands, err = r.commands(resolved.Commands)
		if err != nil {
			return nil, err
		}
		resolved.Ref = ""
		result = append(result, resolved)
	}
	return result, nil
}

func schemaDocument(schemas map[string]json.RawMessage) (*jsonschema.Schema, error) {
	defs := make(map[string]any, len(schemas))
	for name, raw := range schemas {
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("components.schemas.%s: %w", name, err)
		}
		defs[name] = rewriteRefs(value)
	}
	data, err := json.Marshal(map[string]any{"$defs": defs})
	if err != nil {
		return nil, err
	}
	var document jsonschema.Schema
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("assembling schema document: %w", err)
	}
	return &document, nil
}

func rewriteRefs(value any) any {
	switch value := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(value))
		for key, child := range value {
			if key == "$ref" {
				if ref, ok := child.(string); ok && strings.HasPrefix(ref, "#/components/schemas/") {
					result[key] = "#/$defs/" + strings.TrimPrefix(ref, "#/components/schemas/")
					continue
				}
			}
			result[key] = rewriteRefs(child)
		}
		return result
	case []any:
		result := make([]any, len(value))
		for index, child := range value {
			result[index] = rewriteRefs(child)
		}
		return result
	default:
		return value
	}
}
