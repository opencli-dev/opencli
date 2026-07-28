package go_cobra

import (
	"go/parser"
	"go/token"
	"testing"

	genapi "github.com/opencli-dev/opencli/tools/opencli/generator"
	spec "github.com/opencli-dev/opencli/tools/opencli/ir"
)

func TestGoCobraGeneratorProducesGoSource(t *testing.T) {
	s := &spec.IR{
		Info: &spec.Info{Title: "demo"},
		Commands: []spec.Command{{
			Name:        "hello",
			OperationID: "sayHello",
			Flags: []spec.Flag{{
				Name: "format", Type: "string", Choices: []string{"text", "json"},
			}},
		}},
	}

	files, err := (&cobraGenerator{packageName: "generated"}).Generate(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("got %d files, want 3", len(files))
	}

	paths := map[string]bool{}
	for _, file := range files {
		paths[file.Path] = true
		if _, err := parser.ParseFile(token.NewFileSet(), file.Path, file.Content, parser.AllErrors); err != nil {
			t.Errorf("parse %s: %v", file.Path, err)
		}
	}
	for _, path := range []string{"hello.gen.go", "opencli.gen.go", "root.gen.go"} {
		if !paths[path] {
			t.Errorf("missing %s", path)
		}
	}
}

func TestGoCobraIsRegistered(t *testing.T) {
	if _, ok := genapi.Lookup("go-cobra"); !ok {
		t.Fatal("go-cobra generator is not registered")
	}
}
