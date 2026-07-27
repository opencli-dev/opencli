// Package spec is the typed in-memory model (IR) of an OpenCLI specification.
//
// It is the shared foundation for the tooling: the validation pipeline decodes
// a document into a *Spec and runs semantic checks over it, and codegen/docgen
// consume the same structs. Parsing here is purely syntactic — it does not
// validate against the JSON Schema (that is the validate package's job).
package spec

import (
	"encoding/json"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Spec is a complete OpenCLI document.
type Spec struct {
	Schema     string      `json:"$schema,omitempty"`
	OpenCLI    string      `json:"opencli"`
	Info       *Info       `json:"info,omitempty"`
	Flags      []Flag      `json:"flags,omitempty"`
	ExitCodes  []ExitCode  `json:"exitCodes,omitempty"`
	Commands   []Command   `json:"commands,omitempty"`
	Components *Components `json:"components,omitempty"`
}

// Info holds metadata about the CLI tool.
type Info struct {
	Title            string   `json:"title,omitempty"`
	Version          string   `json:"version,omitempty"`
	Description      string   `json:"description,omitempty"`
	LongDescription  string   `json:"longDescription,omitempty"`
	Contact          *Contact `json:"contact,omitempty"`
	License          *License `json:"license,omitempty"`
	Homepage         string   `json:"homepage,omitempty"`
	DocumentationURL string   `json:"documentationUrl,omitempty"`
	BinaryName       string   `json:"binaryName,omitempty"`
}

// Contact is a support contact for the tool.
type Contact struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

// License identifies the tool's license.
type License struct {
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

// Command is a command or subcommand. When Ref is set, it is a reference to a
// reusable command under components.commands and the other fields are empty.
type Command struct {
	Ref string `json:"$ref,omitempty"`

	Name               string      `json:"name,omitempty"`
	OperationID        string      `json:"operationId,omitempty"`
	Aliases            []string    `json:"aliases,omitempty"`
	Description        string      `json:"description,omitempty"`
	LongDescription    string      `json:"longDescription,omitempty"`
	Usage              string      `json:"usage,omitempty"`
	Hidden             bool        `json:"hidden,omitempty"`
	Deprecated         bool        `json:"deprecated,omitempty"`
	DeprecationMessage string      `json:"deprecationMessage,omitempty"`
	Flags              []Flag      `json:"flags,omitempty"`
	Arguments          []Argument  `json:"arguments,omitempty"`
	Commands           []Command   `json:"commands,omitempty"`
	Examples           []Example   `json:"examples,omitempty"`
	EnvVars            []EnvVar    `json:"envVars,omitempty"`
	Tags               []string    `json:"tags,omitempty"`
	FlagGroups         []FlagGroup `json:"flagGroups,omitempty"`
	Stdin              *Stdin      `json:"stdin,omitempty"`
	Output             *Output     `json:"output,omitempty"`
	ExitCodes          []ExitCode  `json:"exitCodes,omitempty"`
}

// IsRef reports whether this command is a reference rather than an inline definition.
func (c Command) IsRef() bool { return c.Ref != "" }

// Flag is a named option. When Ref is set, it is a reference to a reusable flag
// under components.flags and the other fields are empty.
type Flag struct {
	Ref string `json:"$ref,omitempty"`

	Name               string   `json:"name,omitempty"`
	Short              string   `json:"short,omitempty"`
	Description        string   `json:"description,omitempty"`
	LongDescription    string   `json:"longDescription,omitempty"`
	Type               string   `json:"type,omitempty"`
	Format             string   `json:"format,omitempty"`
	Default            any      `json:"default,omitempty"`
	Required           bool     `json:"required,omitempty"`
	Deprecated         bool     `json:"deprecated,omitempty"`
	DeprecationMessage string   `json:"deprecationMessage,omitempty"`
	Hidden             bool     `json:"hidden,omitempty"`
	EnvVar             string   `json:"envVar,omitempty"`
	Choices            []string `json:"choices,omitempty"`
	Placeholder        string   `json:"placeholder,omitempty"`
	Repeatable         bool     `json:"repeatable,omitempty"`
	SplitOnComma       bool     `json:"splitOnComma,omitempty"`
	Count              bool     `json:"count,omitempty"`
	TrackChanged       bool     `json:"trackChanged,omitempty"`
	Sensitive          bool     `json:"sensitive,omitempty"`
}

// IsRef reports whether this flag is a reference rather than an inline definition.
func (f Flag) IsRef() bool { return f.Ref != "" }

// Argument is a positional parameter. Required uses a pointer so an unset value
// (which defaults to true per the schema) is distinguishable from explicit false.
type Argument struct {
	Ref string `json:"$ref,omitempty"`

	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type,omitempty"`
	Format      string   `json:"format,omitempty"`
	Required    *bool    `json:"required,omitempty"`
	Default     any      `json:"default,omitempty"`
	Variadic    bool     `json:"variadic,omitempty"`
	Choices     []string `json:"choices,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Sensitive   bool     `json:"sensitive,omitempty"`
}

// IsRef reports whether this argument is a reference rather than an inline definition.
func (a Argument) IsRef() bool { return a.Ref != "" }

// IsRequired reports the effective required state (defaults to true).
func (a Argument) IsRequired() bool { return a.Required == nil || *a.Required }

// Example is a usage example. When Ref is set, it references components.examples.
type Example struct {
	Ref string `json:"$ref,omitempty"`

	Command     string `json:"command,omitempty"`
	Description string `json:"description,omitempty"`
	Output      string `json:"output,omitempty"`
}

// IsRef reports whether this example is a reference rather than an inline definition.
func (e Example) IsRef() bool { return e.Ref != "" }

// EnvVar documents an environment variable affecting a command.
type EnvVar struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Default     string `json:"default,omitempty"`
}

// ExitCode maps a numeric exit status to a failure mode.
type ExitCode struct {
	Code        int    `json:"code"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
}

// FlagGroup expresses a constraint across a set of flags.
type FlagGroup struct {
	Type        string   `json:"type,omitempty"`
	Flags       []string `json:"flags,omitempty"`
	Description string   `json:"description,omitempty"`
}

// Stdin describes a command's use of standard input.
type Stdin struct {
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Format      string `json:"format,omitempty"`
}

// Output describes a command's output formats.
type Output struct {
	FormatFlag string         `json:"formatFlag,omitempty"`
	Formats    []OutputFormat `json:"formats,omitempty"`
}

// OutputFormat is a single emittable format. Schema holds an arbitrary embedded
// JSON Schema; it is left raw for downstream tools to parse with a real library.
type OutputFormat struct {
	Format      string          `json:"format,omitempty"`
	Value       string          `json:"value,omitempty"`
	Description string          `json:"description,omitempty"`
	Default     bool            `json:"default,omitempty"`
	Schema      json.RawMessage `json:"schema,omitempty"`
}

// Components holds reusable definitions referenced by $ref elsewhere.
type Components struct {
	Flags     map[string]Flag            `json:"flags,omitempty"`
	Arguments map[string]Argument        `json:"arguments,omitempty"`
	Commands  map[string]Command         `json:"commands,omitempty"`
	Examples  map[string]Example         `json:"examples,omitempty"`
	Schemas   map[string]json.RawMessage `json:"schemas,omitempty"`
}

// Issue is a single, human-readable problem located by a JSON Pointer into the
// instance document. Both structural and semantic checks emit Issues.
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

// ToJSON converts a YAML or JSON document to JSON bytes using a YAML 1.2 parser.
// This keeps single-character scalars like n and y as strings rather than
// coercing them to booleans (the YAML 1.1 "Norway problem"), which matters for
// single-letter flag shorts and aliases. JSON input passes through unchanged.
func ToJSON(data []byte) ([]byte, error) {
	var v any
	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

// Parse decodes an OpenCLI document (JSON or YAML) into the typed IR. It is
// syntactic only: it does not check the document against the schema or run
// semantic checks. Use the validate package for the full pipeline.
func Parse(data []byte) (*Spec, error) {
	jsonBytes, err := ToJSON(data)
	if err != nil {
		return nil, err
	}
	var s Spec
	dec := json.NewDecoder(strings.NewReader(string(jsonBytes)))
	if err := dec.Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}
