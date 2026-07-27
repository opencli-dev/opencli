package codegen

import (
	"fmt"
	"strings"

	"github.com/opencli-dev/opencli/tools/opencli/spec"
)

// renderLeafBody writes the body of a leaf (or hybrid leaf-and-group)
// command's constructor: the *cobra.Command literal, flag registration, any
// child commands, and the RunE closure that parses/validates raw flag and
// argument values into a typed Params value before calling the handler.
// allFlags is global flags followed by the command's own flags, in that
// order; globalCount marks the split — global flags are read back via
// cmd.Flags().GetXxx (they're registered once, on the root command) while
// the command's own flags are declared and registered locally. children is
// nil for a pure leaf; for a hybrid node it holds the already-rendered
// subcommands to AddCommand.
func renderLeafBody(w *strings.Builder, c spec.Command, scope string, allFlags []resolvedFlag, args []resolvedArgument, globalCount int, paramVar, returnType string, children []nodeResult, outputType string) {
	min, max := argsMinMax(args)

	w.WriteString("\tcmd := &cobra.Command{\n")
	fmt.Fprintf(w, "\t\tUse:   %s,\n", goStringLiteral(useLine(c, args)))
	if c.Description != "" {
		fmt.Fprintf(w, "\t\tShort: %s,\n", goStringLiteral(c.Description))
	}
	if c.LongDescription != "" {
		fmt.Fprintf(w, "\t\tLong: %s,\n", goRawOrQuoted(c.LongDescription))
	}
	if len(c.Aliases) > 0 {
		fmt.Fprintf(w, "\t\tAliases: %s,\n", goStringSliceLiteral(c.Aliases))
	}
	if ex := renderExamples(c); ex != "" {
		fmt.Fprintf(w, "\t\tExample: %s,\n", goRawOrQuoted(ex))
	}
	fmt.Fprintf(w, "\t\tArgs: rangeArgs(%d, %d),\n", min, max)
	w.WriteString("\t}\n")

	if c.Hidden {
		w.WriteString("\tcmd.Hidden = true\n")
	}
	if c.Deprecated {
		msg := c.DeprecationMessage
		if msg == "" {
			msg = "deprecated"
		}
		fmt.Fprintf(w, "\tcmd.Deprecated = %s\n", goStringLiteral(msg))
	}
	w.WriteString("\n")

	own := allFlags[globalCount:]
	for _, f := range own {
		renderFlagDecl(w, f)
	}
	for _, f := range own {
		if f.Required {
			fmt.Fprintf(w, "\t_ = cmd.MarkFlagRequired(%s)\n", goStringLiteral(f.Name))
		}
		if f.Hidden {
			fmt.Fprintf(w, "\t_ = cmd.Flags().MarkHidden(%s)\n", goStringLiteral(f.Name))
		}
		if f.Deprecated {
			msg := f.DeprecationMessage
			if msg == "" {
				msg = "deprecated"
			}
			fmt.Fprintf(w, "\t_ = cmd.Flags().MarkDeprecated(%s, %s)\n", goStringLiteral(f.Name), goStringLiteral(msg))
		}
	}
	for _, g := range c.FlagGroups {
		names := make([]string, len(g.Flags))
		for i, n := range g.Flags {
			names[i] = goStringLiteral(n)
		}
		switch g.Type {
		case "mutuallyExclusive":
			fmt.Fprintf(w, "\tcmd.MarkFlagsMutuallyExclusive(%s)\n", strings.Join(names, ", "))
		case "requiredTogether":
			fmt.Fprintf(w, "\tcmd.MarkFlagsRequiredTogether(%s)\n", strings.Join(names, ", "))
		case "oneRequired":
			fmt.Fprintf(w, "\tcmd.MarkFlagsOneRequired(%s)\n", strings.Join(names, ", "))
		}
	}

	if len(children) > 0 {
		w.WriteString("\n\tcmd.AddCommand(\n")
		for _, r := range children {
			fmt.Fprintf(w, "\t\t%s,\n", r.CtorExpr)
		}
		w.WriteString("\t)\n")
	}

	needsErr := needsErrVar(allFlags, args)

	var run strings.Builder
	if needsErr {
		run.WriteString("\t\tvar err error\n")
	}

	for _, f := range allFlags {
		if f.TrackChanged {
			fmt.Fprintf(&run, "\t\t%sSet := cmd.Flags().Changed(%s)\n", camelCase(f.FieldName), goStringLiteral(f.Name))
		}
	}
	for _, f := range allFlags {
		if f.EnvVar != "" {
			fmt.Fprintf(&run, "\t\tif !cmd.Flags().Changed(%s) {\n\t\t\tif v := os.Getenv(%s); v != \"\" {\n\t\t\t\t_ = cmd.Flags().Set(%s, v)\n\t\t\t}\n\t\t}\n",
				goStringLiteral(f.Name), goStringLiteral(f.EnvVar), goStringLiteral(f.Name))
		}
	}

	fieldValues := map[string]string{}

	for i, f := range allFlags {
		isGlobal := i < globalCount
		fieldValues[f.FieldName] = renderFlagRead(&run, f, isGlobal)
	}

	for i, a := range args {
		fieldValues[a.FieldName] = renderArgRead(&run, a, i)
	}

	fmt.Fprintf(&run, "\n\t\tp := %sParams{\n", scope)
	for _, f := range allFlags {
		fmt.Fprintf(&run, "\t\t\t%s: %s,\n", f.FieldName, fieldValues[f.FieldName])
		if f.TrackChanged {
			fmt.Fprintf(&run, "\t\t\t%sSet: %sSet,\n", f.FieldName, camelCase(f.FieldName))
		}
	}
	for _, a := range args {
		fmt.Fprintf(&run, "\t\t\t%s: %s,\n", a.FieldName, fieldValues[a.FieldName])
	}
	run.WriteString("\t\t}\n\n")
	renderHandlerCall(&run, c, allFlags, paramVar, outputType, fieldValues)

	w.WriteString("\n\tcmd.RunE = func(cmd *cobra.Command, args []string) error {\n")
	w.WriteString(run.String())
	w.WriteString("\t}\n\n")
	fmt.Fprintf(w, "\treturn %s\n", castTo(returnType, "cmd"))
}

// renderHandlerCall writes the final statement(s) of a leaf's RunE: calling
// the Handler, and, for a command with a resolved output type, encoding the
// result for whichever formats declared a schema. fieldValues is the same
// flag-name -> local-variable-expression map used to build the Params
// literal, so the format flag's already-parsed enum value can be reused
// directly instead of re-deriving it.
func renderHandlerCall(run *strings.Builder, c spec.Command, allFlags []resolvedFlag, paramVar, outputType string, fieldValues map[string]string) {
	if outputType == "" {
		fmt.Fprintf(run, "\t\treturn %s(cmd.Context(), cmd, newIO(cmd), p)\n", paramVar)
		return
	}

	fmt.Fprintf(run, "\t\tresult, err := %s(cmd.Context(), cmd, newIO(cmd), p)\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n", paramVar)

	formatVar := ""
	enumType := ""
	if c.Output.FormatFlag != "" {
		for _, f := range allFlags {
			if f.Name == c.Output.FormatFlag {
				formatVar = fieldValues[f.FieldName]
				enumType = f.EnumType
			}
		}
	}

	type schemaFormat struct {
		enumConst string
		encoding  string // "json" or "yaml" — the only encodings codegen can auto-serialize
	}
	var schemaFormats []schemaFormat
	for _, f := range c.Output.Formats {
		// Mirrors resolveOutputType's filter exactly — only json/yaml with a
		// resolvable $ref contributed to outputType, so only those get an
		// auto-encode case here.
		if f.Format != "json" && f.Format != "yaml" {
			continue
		}
		if ref, _ := schemaRef(f.Schema); ref == "" {
			continue
		}
		schemaFormats = append(schemaFormats, schemaFormat{
			enumConst: enumType + pascalCase(f.Format),
			encoding:  f.Format,
		})
	}

	if formatVar == "" {
		// No --format flag was generated at all, meaning this command has
		// exactly one declared output format — nothing to switch on.
		run.WriteString(indent(encodeStmt(schemaFormats[0].encoding), "\t\t"))
		return
	}

	fmt.Fprintf(run, "\t\tswitch %s {\n", formatVar)
	for _, sf := range schemaFormats {
		fmt.Fprintf(run, "\t\tcase %s:\n%s", sf.enumConst, indent(encodeStmt(sf.encoding), "\t\t\t"))
	}
	run.WriteString("\t\t}\n\t\treturn nil\n")
}

// encodeStmt returns the (unindented) statements that encode `result` and
// return, for one of the two encodings codegen knows how to auto-serialize.
func encodeStmt(encoding string) string {
	if encoding == "yaml" {
		return "enc := yaml.NewEncoder(cmd.OutOrStdout())\n" +
			"if err := enc.Encode(result); err != nil {\n\treturn err\n}\n" +
			"return enc.Close()\n"
	}
	return "return json.NewEncoder(cmd.OutOrStdout()).Encode(result)\n"
}

func indent(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n") + "\n"
}

// useLine builds the Cobra Use string: the command's explicit Usage
// override if set, otherwise the command name followed by a placeholder for
// each positional argument.
func useLine(c spec.Command, args []resolvedArgument) string {
	if c.Usage != "" {
		return c.Usage
	}
	parts := []string{c.Name}
	for _, a := range args {
		parts = append(parts, placeholder(a.Name, a.Placeholder, a.IsRequired(), a.Variadic))
	}
	return strings.Join(parts, " ")
}

// renderExamples flattens a command's examples into Cobra's single Example
// string, one example per line (with an optional "# description" line above
// commands that document one).
func renderExamples(c spec.Command) string {
	if len(c.Examples) == 0 {
		return ""
	}
	var lines []string
	for _, ex := range c.Examples {
		if ex.Description != "" {
			for descLine := range strings.SplitSeq(strings.TrimRight(ex.Description, "\n"), "\n") {
				lines = append(lines, "  # "+descLine)
			}
		}
		lines = append(lines, "  "+ex.Command)
	}
	return strings.Join(lines, "\n")
}

func needsErrVar(flags []resolvedFlag, args []resolvedArgument) bool {
	for _, f := range flags {
		if f.Kind == kindUUID {
			return true
		}
	}
	for _, a := range args {
		if a.Kind == kindUUID {
			return true
		}
	}
	return false
}

// argsMinMax computes the (min, max) argument counts for cobra's
// PositionalArgs check. Required arguments must precede optional ones and a
// variadic argument must be last (enforced by opencli lint), so min is just
// the count of required arguments and max is -1 (unbounded) whenever the
// last argument is variadic.
func argsMinMax(args []resolvedArgument) (min, max int) {
	for _, a := range args {
		if a.IsRequired() {
			min++
		}
	}
	max = len(args)
	if len(args) > 0 && args[len(args)-1].Variadic {
		max = -1
	}
	return min, max
}

// renderFlagDecl emits the local variable declaration and pflag registration
// for one of a command's own flags (never called for global flags, which are
// registered once on the root command).
func renderFlagDecl(w *strings.Builder, f resolvedFlag) {
	raw := "raw" + f.FieldName
	def := goDefaultLiteral(f.Kind, f.Default)
	usage := f.Description

	fmt.Fprintf(w, "\tvar %s %s\n", raw, f.Kind.rawGoType())
	fn := f.Kind.pflagVarFunc()
	if f.Short != "" {
		fmt.Fprintf(w, "\tcmd.Flags().%sP(&%s, %s, %s, %s, %s)\n",
			fn, raw, goStringLiteral(f.Name), goStringLiteral(f.Short), def, goStringLiteral(usage))
	} else {
		fmt.Fprintf(w, "\tcmd.Flags().%s(&%s, %s, %s, %s)\n",
			fn, raw, goStringLiteral(f.Name), def, goStringLiteral(usage))
	}
}

// renderFlagRead writes to run whatever statements are needed to produce the
// final typed value for f (reading a local raw variable for the command's
// own flags, or calling cmd.Flags().GetXxx for an inherited global flag),
// returning the Go expression that yields that value for the Params literal.
func renderFlagRead(run *strings.Builder, f resolvedFlag, isGlobal bool) string {
	field := camelCase(f.FieldName)

	var raw string
	if isGlobal {
		raw = "raw" + f.FieldName
		fmt.Fprintf(run, "\t\t%s, _ := cmd.Flags().%s(%s)\n", raw, f.Kind.pflagGetFunc(), goStringLiteral(f.Name))
	} else {
		raw = "raw" + f.FieldName
	}

	switch f.Kind {
	case kindUUID:
		fmt.Fprintf(run, "\t\tvar %s uuid.UUID\n\t\tif %s != \"\" {\n\t\t\t%s, err = uuid.Parse(%s)\n\t\t\tif err != nil {\n\t\t\t\treturn fmt.Errorf(%s, err)\n\t\t\t}\n\t\t}\n",
			field, raw, field, raw, goStringLiteral("invalid --"+f.Name+": %w"))
		return field
	case kindEnum:
		fmt.Fprintf(run, "\t\tvar %s %s\n\t\tif %s != \"\" {\n\t\t\tswitch %s {\n", field, f.EnumType, raw, raw)
		for _, c := range f.Choices {
			fmt.Fprintf(run, "\t\t\tcase %s:\n\t\t\t\t%s = %s%s\n", goStringLiteral(c), field, f.EnumType, pascalCase(c))
		}
		fmt.Fprintf(run, "\t\t\tdefault:\n\t\t\t\treturn fmt.Errorf(%s, %s, %s)\n\t\t\t}\n\t\t}\n",
			goStringLiteral("invalid --"+f.Name+" %q: must be one of "+strings.Join(f.Choices, ", ")), raw, "")
		return field
	default:
		if isGlobal {
			return raw
		}
		return raw
	}
}

// renderArgRead writes to run whatever statements are needed to extract
// argument a (at position i of total) from the positional args slice and
// produce its final typed value, returning the Go expression for the Params
// literal.
func renderArgRead(run *strings.Builder, a resolvedArgument, i int) string {
	field := camelCase(a.FieldName)
	raw := "raw" + a.FieldName

	switch {
	case a.Variadic:
		fmt.Fprintf(run, "\t\t%s := args[%d:]\n", field, i)
		return field
	case a.IsRequired():
		fmt.Fprintf(run, "\t\t%s := args[%d]\n", raw, i)
	default:
		fmt.Fprintf(run, "\t\tvar %s string\n\t\tif len(args) > %d {\n\t\t\t%s = args[%d]\n\t\t}\n", raw, i, raw, i)
	}

	switch a.Kind {
	case kindUUID:
		fmt.Fprintf(run, "\t\tvar %s uuid.UUID\n\t\tif %s != \"\" {\n\t\t\t%s, err = uuid.Parse(%s)\n\t\t\tif err != nil {\n\t\t\t\treturn fmt.Errorf(%s, err)\n\t\t\t}\n\t\t}\n",
			field, raw, field, raw, goStringLiteral("invalid "+a.Name+": %w"))
		return field
	case kindEnum:
		fmt.Fprintf(run, "\t\tvar %s %s\n\t\tif %s != \"\" {\n\t\t\tswitch %s {\n", field, a.EnumType, raw, raw)
		for _, c := range a.Choices {
			fmt.Fprintf(run, "\t\t\tcase %s:\n\t\t\t\t%s = %s%s\n", goStringLiteral(c), field, a.EnumType, pascalCase(c))
		}
		fmt.Fprintf(run, "\t\t\tdefault:\n\t\t\t\treturn fmt.Errorf(%s, %s)\n\t\t\t}\n\t\t}\n",
			goStringLiteral("invalid "+a.Name+" %q: must be one of "+strings.Join(a.Choices, ", ")), raw)
		return field
	default:
		return raw
	}
}
