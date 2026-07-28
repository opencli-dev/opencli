// Package rust_clap implements the rust-clap OpenCLI generator.
package rust_clap

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"text/template"

	"github.com/Southclaws/schemancer/schemancer/generators"
	schemancerrust "github.com/Southclaws/schemancer/schemancer/generators/rust"
	schemancerir "github.com/Southclaws/schemancer/schemancer/ir"
	"github.com/spf13/pflag"

	genapi "github.com/opencli-dev/opencli/tools/opencli/generator"
	spec "github.com/opencli-dev/opencli/tools/opencli/ir"
)

const tag = "rust-clap"

func init() {
	genapi.Register(tag, func() genapi.Generator { return &generator{module: "opencli_gen"} })
}

type generator struct{ module string }

func (*generator) Short() string { return "Generate Rust command scaffolding using Clap" }

func (*generator) Long() string {
	return "Generate a typed Rust module using Clap's derive API, including nested subcommands, value enums, argument relationships, and schema types."
}

func (g *generator) ConfigureFlags(flags *pflag.FlagSet) {
	flags.StringVar(&g.module, "module", "opencli_gen", "Rust module and output filename (without .rs)")
}

//go:embed templates/*.tmpl
var templateFiles embed.FS

var templates = template.Must(template.New("rust-clap").ParseFS(templateFiles, "templates/*.tmpl"))

type templateData struct {
	RootAttributes string
	Globals        []field
	Commands       []command
	Enums          []enum
	NeedsUUID      bool
	NeedsDuration  bool
	NeedsSchemas   bool
}

type command struct {
	Variant        string
	ArgsType       string
	SubcommandType string
	Attributes     string
	Fields         []field
	Groups         []string
	Children       []command
	Runnable       bool
	OperationID    string
}

type field struct {
	Name       string
	Type       string
	Attributes string
}

type enum struct {
	Name   string
	Values []enumValue
}

type enumValue struct {
	Name    string
	Variant string
}

func (g *generator) Generate(input *spec.IR) ([]genapi.GeneratedFile, error) {
	data := prepare(input)
	data.NeedsSchemas = input.Schema != nil && len(input.Schema.Types) > 0

	var source bytes.Buffer
	if err := templates.ExecuteTemplate(&source, "file", data); err != nil {
		return nil, fmt.Errorf("rust-clap: execute template: %w", err)
	}
	if input.Schema != nil && len(input.Schema.Types) > 0 {
		types, err := generateSchemaTypes(input.Schema)
		if err != nil {
			return nil, err
		}
		source.WriteString("\n")
		source.Write(types)
	}

	formatted, err := rustfmt(source.Bytes())
	if err != nil {
		return nil, err
	}
	return []genapi.GeneratedFile{{Path: snake(g.module) + ".rs", Content: formatted}}, nil
}

func prepare(input *spec.IR) templateData {
	root := []string{"name = " + rustString(commandName(input))}
	if input.Info != nil {
		if input.Info.Version != "" {
			root = append(root, "version = "+rustString(input.Info.Version))
		}
		if input.Info.Description != "" {
			root = append(root, "about = "+rustString(input.Info.Description))
		}
		if input.Info.LongDescription != "" {
			root = append(root, "long_about = "+rustString(input.Info.LongDescription))
		}
	}

	data := templateData{RootAttributes: strings.Join(root, ", ")}
	enums := map[string]enum{}
	for _, value := range input.Flags {
		data.Globals = append(data.Globals, makeFlag(value, "Global", true, enums, &data))
	}
	for _, value := range input.Commands {
		data.Commands = append(data.Commands, makeCommand(value, nil, enums, &data))
	}
	for _, value := range enums {
		data.Enums = append(data.Enums, value)
	}
	// Enum discovery follows spec order, but map collection does not. Keep
	// source reproducible by sorting after collection.
	sortEnums(data.Enums)
	return data
}

func commandName(input *spec.IR) string {
	if input.Info != nil {
		if input.Info.BinaryName != "" {
			return input.Info.BinaryName
		}
		if input.Info.Title != "" {
			return input.Info.Title
		}
	}
	return "cli"
}

func makeCommand(value spec.Command, parents []string, enums map[string]enum, data *templateData) command {
	path := append(append([]string{}, parents...), value.Name)
	typePrefix := ""
	for _, part := range path {
		typePrefix += pascal(part)
	}
	result := command{
		Variant: pascal(value.Name), ArgsType: typePrefix + "Args",
		SubcommandType: typePrefix + "Command", Runnable: value.OperationID != "",
		OperationID: rustString(value.OperationID), Attributes: commandAttributes(value),
	}
	for _, flagValue := range value.Flags {
		result.Fields = append(result.Fields, makeFlag(flagValue, typePrefix, false, enums, data))
	}
	for index, argumentValue := range value.Arguments {
		result.Fields = append(result.Fields, makeArgument(argumentValue, index, typePrefix, enums, data))
	}
	applyRequiredTogether(result.Fields, value.FlagGroups)
	result.Groups = makeGroups(value.FlagGroups)
	for _, child := range value.Commands {
		result.Children = append(result.Children, makeCommand(child, path, enums, data))
	}
	return result
}

func commandAttributes(value spec.Command) string {
	attributes := []string{"name = " + rustString(value.Name)}
	if value.Description != "" {
		attributes = append(attributes, "about = "+rustString(value.Description))
	}
	long := value.LongDescription
	if value.Deprecated {
		message := value.DeprecationMessage
		if message == "" {
			message = "deprecated"
		}
		long = strings.TrimSpace(long + "\n\nDeprecated: " + message)
	}
	if long != "" {
		attributes = append(attributes, "long_about = "+rustString(long))
	}
	if value.Usage != "" {
		attributes = append(attributes, "override_usage = "+rustString(value.Usage))
	}
	if value.Hidden {
		attributes = append(attributes, "hide = true")
	}
	if len(value.Aliases) > 0 {
		aliases := make([]string, len(value.Aliases))
		for index, alias := range value.Aliases {
			aliases[index] = rustString(alias)
		}
		attributes = append(attributes, "visible_aliases = ["+strings.Join(aliases, ", ")+"]")
	}
	if examples := examples(value.Examples); examples != "" {
		attributes = append(attributes, "after_help = "+rustString(examples))
	}
	return strings.Join(attributes, ", ")
}

func examples(values []spec.Example) string {
	var lines []string
	for _, value := range values {
		if value.Description != "" {
			lines = append(lines, value.Description)
		}
		lines = append(lines, "  "+value.Command)
	}
	return strings.Join(lines, "\n")
}

func makeFlag(value spec.Flag, scope string, global bool, enums map[string]enum, data *templateData) field {
	attributes := []string{"id = " + rustString(value.Name), "long = " + rustString(value.Name)}
	if value.Short != "" {
		attributes = append(attributes, "short = "+strconv.QuoteRune([]rune(value.Short)[0]))
	}
	if value.Description != "" {
		attributes = append(attributes, "help = "+rustString(value.Description))
	}
	if global {
		attributes = append(attributes, "global = true")
	}
	if value.EnvVar != "" {
		attributes = append(attributes, "env = "+rustString(value.EnvVar))
	}
	if value.Sensitive {
		attributes = append(attributes, "hide_env_values = true")
	}
	if value.Hidden {
		attributes = append(attributes, "hide = true")
	}
	if value.Required {
		attributes = append(attributes, "required = true")
	}
	if value.Count {
		attributes = append(attributes, "action = clap::ArgAction::Count")
	}
	if value.SplitOnComma {
		attributes = append(attributes, "value_delimiter = ','")
	}
	if value.Default != nil {
		attributes = append(attributes, "default_value = "+rustString(defaultString(value.Default)))
	}
	if value.Deprecated {
		message := value.DeprecationMessage
		if message == "" {
			message = "deprecated"
		}
		attributes = append(attributes, "long_help = "+rustString(strings.TrimSpace(value.Description+"\n\nDeprecated: "+message)))
	}

	typeName := scalarType(value.Type, value.Format, data)
	if len(value.Choices) > 0 {
		typeName = addEnum(scope+pascal(value.Name), value.Choices, enums)
		attributes = append(attributes, "value_enum")
	}
	switch {
	case value.Count:
		typeName = "u8"
	case value.Repeatable:
		typeName = "Vec<" + typeName + ">"
	case value.Type == "boolean":
		typeName = "bool"
	case !value.Required && value.Default == nil:
		typeName = "Option<" + typeName + ">"
	}
	return field{Name: snake(value.Name), Type: typeName, Attributes: strings.Join(attributes, ", ")}
}

func makeArgument(value spec.Argument, index int, scope string, enums map[string]enum, data *templateData) field {
	attributes := []string{"id = " + rustString(value.Name), "index = " + strconv.Itoa(index+1)}
	if value.Description != "" {
		attributes = append(attributes, "help = "+rustString(value.Description))
	}
	if value.Placeholder != "" {
		attributes = append(attributes, "value_name = "+rustString(value.Placeholder))
	}
	if value.Default != nil {
		attributes = append(attributes, "default_value = "+rustString(defaultString(value.Default)))
	}
	typeName := scalarType(value.Type, value.Format, data)
	if len(value.Choices) > 0 {
		typeName = addEnum(scope+pascal(value.Name), value.Choices, enums)
		attributes = append(attributes, "value_enum")
	}
	if value.Variadic {
		typeName = "Vec<" + typeName + ">"
	} else if !value.IsRequired() && value.Default == nil {
		typeName = "Option<" + typeName + ">"
	}
	return field{Name: snake(value.Name), Type: typeName, Attributes: strings.Join(attributes, ", ")}
}

func scalarType(typeName, format string, data *templateData) string {
	switch format {
	case "uuid":
		data.NeedsUUID = true
		return "uuid::Uuid"
	case "duration":
		data.NeedsDuration = true
		return "humantime::Duration"
	}
	switch typeName {
	case "integer":
		return "i64"
	case "number":
		return "f64"
	case "boolean":
		return "bool"
	default:
		return "String"
	}
}

func addEnum(name string, choices []string, enums map[string]enum) string {
	if _, exists := enums[name]; !exists {
		value := enum{Name: name}
		seen := map[string]int{}
		for _, choice := range choices {
			variant := pascal(choice)
			seen[variant]++
			if seen[variant] > 1 {
				variant += strconv.Itoa(seen[variant])
			}
			value.Values = append(value.Values, enumValue{Name: rustString(choice), Variant: variant})
		}
		enums[name] = value
	}
	return name
}

func makeGroups(values []spec.FlagGroup) []string {
	var groups []string
	for index, value := range values {
		if len(value.Flags) == 0 || value.Type == "requiredTogether" {
			continue
		}
		args := make([]string, len(value.Flags))
		for flagIndex, flagName := range value.Flags {
			args[flagIndex] = rustString(flagName)
		}
		parts := []string{
			"id = " + rustString(fmt.Sprintf("opencli_group_%d", index)),
			"args = [" + strings.Join(args, ", ") + "]",
		}
		switch value.Type {
		case "mutuallyExclusive":
			parts = append(parts, "multiple = false")
		case "oneRequired":
			parts = append(parts, "required = true", "multiple = true")
		}
		groups = append(groups, strings.Join(parts, ", "))
	}
	return groups
}

func applyRequiredTogether(fields []field, groups []spec.FlagGroup) {
	byName := make(map[string]int, len(fields))
	for index, value := range fields {
		byName[value.Name] = index
	}
	for _, group := range groups {
		if group.Type != "requiredTogether" {
			continue
		}
		for _, flagName := range group.Flags {
			index, ok := byName[snake(flagName)]
			if !ok {
				continue
			}
			var others []string
			for _, other := range group.Flags {
				if other != flagName {
					others = append(others, rustString(other))
				}
			}
			if len(others) > 0 {
				fields[index].Attributes += ", requires_all = [" + strings.Join(others, ", ") + "]"
			}
		}
	}
}

func defaultString(value any) string {
	switch value := value.(type) {
	case string:
		return value
	case bool:
		return strconv.FormatBool(value)
	case float64:
		return strconv.FormatFloat(value, 'g', -1, 64)
	default:
		return fmt.Sprint(value)
	}
}

func sortEnums(values []enum) {
	for left := 0; left < len(values); left++ {
		for right := left + 1; right < len(values); right++ {
			if values[right].Name < values[left].Name {
				values[left], values[right] = values[right], values[left]
			}
		}
	}
}

func rustfmt(source []byte) ([]byte, error) {
	executable, err := exec.LookPath("rustfmt")
	if errors.Is(err, exec.ErrNotFound) {
		return source, nil
	}
	if err != nil {
		return nil, fmt.Errorf("rust-clap: locate rustfmt: %w", err)
	}

	command := exec.Command(executable, "--emit", "stdout", "--edition", "2024")
	command.Stdin = bytes.NewReader(source)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	formatted, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("rust-clap: format generated Rust: %w: %s\n%s", err, strings.TrimSpace(stderr.String()), source)
	}
	return formatted, nil
}

func generateSchemaTypes(schema *schemancerir.IR) ([]byte, error) {
	files, err := (&schemancerrust.Generator{}).Generate(
		schema,
		generators.GeneratorOptions{},
		schemancerrust.WithFilename("types.rs"),
	)
	if err != nil {
		return nil, fmt.Errorf("rust-clap: generate schema types: %w", err)
	}
	if len(files) != 1 {
		return nil, fmt.Errorf("rust-clap: expected one schema file, got %d", len(files))
	}
	return files[0].Content, nil
}
