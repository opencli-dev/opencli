package rust_clap

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	genapi "github.com/opencli-dev/opencli/tools/opencli/generator"
	opencliir "github.com/opencli-dev/opencli/tools/opencli/ir"
	"github.com/opencli-dev/opencli/tools/opencli/spec"
)

func TestGenerate(t *testing.T) {
	input := &opencliir.IR{
		Info:  &opencliir.Info{Title: "demo", Version: "1.2.3"},
		Flags: []opencliir.Flag{{Name: "verbose", Short: "v", Count: true}},
		Commands: []opencliir.Command{{
			Name:  "item",
			Flags: []opencliir.Flag{{Name: "output", Type: "path"}},
			Commands: []opencliir.Command{{
				Name: "get", OperationID: "itemGet",
				Arguments: []opencliir.Argument{{Name: "ITEM_ID", Format: "uuid"}},
				Flags:     []opencliir.Flag{{Name: "format", Choices: []string{"json", "yaml"}}},
			}},
		}},
	}

	files, err := (&generator{module: "generated_cli"}).Generate(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "generated_cli.rs" {
		t.Fatalf("unexpected files: %#v", files)
	}
	source := string(files[0].Content)
	for _, expected := range []string{
		"pub struct Cli", "pub enum ItemCommand", "pub struct ItemGetArgs",
		`pub const OPERATION_ID: &'static str = r"itemGet"`, "uuid::Uuid", "ValueEnum",
		"pub trait Handler", "async fn item_get", "pub async fn dispatch", "std::path::PathBuf",
		"handler.item_get(",
	} {
		if !strings.Contains(source, expected) {
			t.Errorf("generated source does not contain %q", expected)
		}
	}
}

func TestGenerateDispatchesRunnableParentsAndNestedCommands(t *testing.T) {
	input := &opencliir.IR{
		Info:  &opencliir.Info{Title: "demo", Version: "1.2.3"},
		Flags: []opencliir.Flag{{Name: "verbose", Type: "boolean"}},
		Commands: []opencliir.Command{
			{Name: "serve", OperationID: "serveDatabase"},
			{
				Name: "schema", OperationID: "inspectSchema",
				Commands: []opencliir.Command{{Name: "diff", OperationID: "schemaDiff"}},
			},
		},
	}

	files, err := (&generator{module: "generated_cli"}).Generate(input)
	if err != nil {
		t.Fatal(err)
	}
	source := string(files[0].Content)
	for _, expected := range []string{
		"pub struct GlobalArgs",
		"async fn serve_database(",
		"args: ServeArgs",
		"async fn inspect_schema(",
		"args: SchemaOptions",
		"async fn schema_diff(",
		"args: SchemaDiffArgs",
		"context_schema: &SchemaOptions",
		"Some(command) => command",
		"None => handler.inspect_schema(",
		"self.options",
	} {
		if !strings.Contains(source, expected) {
			t.Errorf("generated source does not contain %q", expected)
		}
	}
}

func TestGenerateUsesSchemancerForCompleteJSONSchemaSupport(t *testing.T) {
	model, err := opencliir.Build(&spec.Spec{
		Components: &spec.Components{Schemas: map[string]json.RawMessage{
			"Event": json.RawMessage(`{
				"oneOf": [
					{"$ref": "#/components/schemas/DataEvent"},
					{"$ref": "#/components/schemas/EmptyEvent"}
				]
			}`),
			"DataEvent": json.RawMessage(`{
				"type": "object",
				"required": ["kind", "payload"],
				"properties": {
					"kind": {"type": "string", "const": "data"},
					"payload": {"type": "string"}
				}
			}`),
			"EmptyEvent": json.RawMessage(`{
				"type": "object",
				"required": ["kind"],
				"properties": {"kind": {"type": "string", "const": "empty"}}
			}`),
			"Priority": json.RawMessage(`{"type":"integer","enum":[1,2]}`),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	files, err := (&generator{module: "schema_types"}).Generate(model)
	if err != nil {
		t.Fatal(err)
	}
	generated := string(files[0].Content)
	for _, expected := range []string{"pub enum Event", "pub enum Priority", "Serialize", "Deserialize"} {
		if !strings.Contains(generated, expected) {
			t.Errorf("generated source does not contain %q", expected)
		}
	}
}

func TestRegistrationAndFlags(t *testing.T) {
	registered, ok := genapi.Lookup("rust-clap")
	if !ok {
		t.Fatal("rust-clap generator is not registered")
	}
	flags := pflag.NewFlagSet("rust-clap", pflag.ContinueOnError)
	registered.ConfigureFlags(flags)
	if flags.Lookup("module") == nil {
		t.Fatal("rust-clap did not register --module")
	}
	if flags.Lookup("package") != nil {
		t.Fatal("rust-clap unexpectedly registered --package")
	}
}

func TestRustfmtIsOptional(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	source := []byte("pub struct Unformatted{pub value:String}")
	generated, err := rustfmt(source)
	if err != nil {
		t.Fatal(err)
	}
	if string(generated) != string(source) {
		t.Fatalf("source changed without rustfmt: %q", generated)
	}
}
