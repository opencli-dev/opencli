package cli

import (
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestGeneratedFilePath(t *testing.T) {
	output := t.TempDir()
	want := filepath.Join(output, "commands", "root.go")
	got, err := generatedFilePath(output, "commands/root.go")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	for _, path := range []string{"", ".", "..", "../outside.go", "/outside.go"} {
		t.Run(path, func(t *testing.T) {
			if _, err := generatedFilePath(output, path); err == nil {
				t.Fatalf("expected %q to be rejected", path)
			}
		})
	}
}

func TestGeneratorCommandsOwnTheirFlags(t *testing.T) {
	cmd := (*cobra.Command)(NewGenerateCommand(generateGoCobraHandler, generateRustClapHandler))
	goCommand, _, err := cmd.Find([]string{"go-cobra"})
	if err != nil {
		t.Fatal(err)
	}
	rustCommand, _, err := cmd.Find([]string{"rust-clap"})
	if err != nil {
		t.Fatal(err)
	}
	if goCommand.Flags().Lookup("package") == nil || goCommand.Flags().Lookup("module") != nil {
		t.Fatal("go-cobra flags are not isolated")
	}
	if rustCommand.Flags().Lookup("module") == nil || rustCommand.Flags().Lookup("package") != nil {
		t.Fatal("rust-clap flags are not isolated")
	}
}
