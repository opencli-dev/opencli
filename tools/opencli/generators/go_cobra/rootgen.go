package go_cobra

import (
	"fmt"
	"strings"

	spec "github.com/opencli-dev/opencli/tools/opencli/ir"
)

// topEntry is one top-level command as seen by the root assembler: the
// constructor parameter name/type it contributes and the expression that
// builds it.
type topEntry struct {
	VarName  string
	TypeName string
}

// renderRoot writes NewRootCommand: it registers the spec's global flags as
// persistent flags (so every subcommand inherits them, unlike the
// hand-written root flag they replace) and assembles the top-level command
// tree from already-constructed per-group values.
func renderRoot(w *strings.Builder, s *spec.IR, globalFlags []resolvedFlag, tops []topEntry) error {
	title := "cli"
	if s.Info != nil && s.Info.Title != "" {
		title = s.Info.Title
	}

	data := rootTemplateData{Use: goStringLiteral(title), Parameters: tops}
	if s.Info != nil && s.Info.Description != "" {
		data.Short = goStringLiteral(s.Info.Description)
	}
	if s.Info != nil && s.Info.LongDescription != "" {
		data.Long = goRawOrQuoted(s.Info.LongDescription)
	}

	for _, f := range globalFlags {
		def := goDefaultLiteral(f.Kind, f.Default)
		fn := f.Kind.pflagPlainFunc()
		if f.Short != "" {
			data.PersistentFlags = append(data.PersistentFlags, fmt.Sprintf("cmd.PersistentFlags().%sP(%s, %s, %s, %s)",
				fn, goStringLiteral(f.Name), goStringLiteral(f.Short), def, goStringLiteral(f.Description)))
		} else {
			data.PersistentFlags = append(data.PersistentFlags, fmt.Sprintf("cmd.PersistentFlags().%s(%s, %s, %s)",
				fn, goStringLiteral(f.Name), def, goStringLiteral(f.Description)))
		}
	}
	source, err := executeTemplate("root", data)
	if err != nil {
		return err
	}
	w.WriteString(source)
	return nil
}

type rootTemplateData struct {
	Use             string
	Short           string
	Long            string
	Parameters      []topEntry
	PersistentFlags []string
}
