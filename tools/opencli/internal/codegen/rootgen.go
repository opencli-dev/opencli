package codegen

import (
	"fmt"
	"strings"

	"github.com/opencli-dev/opencli/tools/opencli/spec"
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
func renderRoot(w *strings.Builder, s *spec.Spec, globalFlags []resolvedFlag, tops []topEntry) {
	sig := make([]string, len(tops))
	for i, t := range tops {
		sig[i] = fmt.Sprintf("%s %s", t.VarName, t.TypeName)
	}

	title := "cli"
	if s.Info != nil && s.Info.Title != "" {
		title = s.Info.Title
	}

	fmt.Fprintf(w, "\nfunc NewRootCommand(\n\t%s,\n) *cobra.Command {\n", strings.Join(sig, ",\n\t"))

	w.WriteString("\tcmd := &cobra.Command{\n")
	fmt.Fprintf(w, "\t\tUse:   %s,\n", goStringLiteral(title))
	if s.Info != nil && s.Info.Description != "" {
		fmt.Fprintf(w, "\t\tShort: %s,\n", goStringLiteral(s.Info.Description))
	}
	if s.Info != nil && s.Info.LongDescription != "" {
		fmt.Fprintf(w, "\t\tLong: %s,\n", goRawOrQuoted(s.Info.LongDescription))
	}
	w.WriteString("\t}\n\n")

	for _, f := range globalFlags {
		def := goDefaultLiteral(f.Kind, f.Default)
		fn := f.Kind.pflagPlainFunc()
		if f.Short != "" {
			fmt.Fprintf(w, "\tcmd.PersistentFlags().%sP(%s, %s, %s, %s)\n",
				fn, goStringLiteral(f.Name), goStringLiteral(f.Short), def, goStringLiteral(f.Description))
		} else {
			fmt.Fprintf(w, "\tcmd.PersistentFlags().%s(%s, %s, %s)\n",
				fn, goStringLiteral(f.Name), def, goStringLiteral(f.Description))
		}
	}
	w.WriteString("\n\tcmd.AddCommand(\n")
	for _, t := range tops {
		fmt.Fprintf(w, "\t\t(*cobra.Command)(%s),\n", t.VarName)
	}
	w.WriteString("\t)\n\n\treturn cmd\n}\n")
}
