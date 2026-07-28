package go_cobra

import (
	"fmt"
	"strings"

	spec "github.com/opencli-dev/opencli/tools/opencli/ir"
)

// generator holds state shared across every file rendered for one spec.
type generator struct {
	globalFlags []resolvedFlag
}

// handlerParam is one (paramName, HandlerTypeName) pair threaded through a
// constructor's signature — the flattened list of every leaf handler needed
// by that constructor's subtree.
type handlerParam struct {
	VarName  string
	TypeName string
}

// nodeResult is what rendering one command node hands back to its caller:
// the expression its parent uses to invoke its constructor (e.g.
// "newFlowVersionCommand(versionList, versionCreate)"), and the flattened
// handler parameters that constructor requires.
type nodeResult struct {
	CtorExpr string
	Params   []handlerParam
}

// renderNode renders one command (and, for a group, its entire subtree) into
// w, returning how its parent should invoke it. namePrefix accumulates the
// PascalCase path for nested (non-top-level) groups, which have no
// operationId and so need their ancestry folded into their generated names
// to stay unique package-wide (operationIds are already globally unique per
// spec lint, so leaves never need a prefix).
//
// A command may be a pure leaf (operationId, no children), a pure group (no
// operationId, children only), or both at once — e.g. `info` is runnable
// itself (its own --format flag) and also has an `info metadata` child. The
// hybrid case renders its own Params/Handler exactly like a leaf, and its
// constructor both sets RunE and calls AddCommand on its children.
func (g *generator) renderNode(w *strings.Builder, c spec.Command, topLevel bool, namePrefix string) (nodeResult, error) {
	switch {
	case len(c.Commands) == 0:
		return g.renderLeaf(w, c, topLevel)
	case c.OperationID == "":
		return g.renderGroup(w, c, topLevel, namePrefix, nil)
	default:
		return g.renderHybrid(w, c, topLevel, namePrefix)
	}
}

// ownFlagsAndArgs computes a command's own effective flag set (its declared
// flags plus any global flags not shadowed by a same-named local one) and
// its classified arguments, and renders their enum types and Params struct.
// Shared between the pure-leaf and hybrid render paths, which both have a
// "self" Handler/Params to emit.
func (g *generator) ownFlagsAndArgs(w *strings.Builder, c spec.Command, scope string) (allFlags []resolvedFlag, args []resolvedArgument, effectiveGlobalCount int, outputType string, err error) {
	ownFlags := make([]resolvedFlag, len(c.Flags))
	ownNames := make(map[string]bool, len(c.Flags))
	for i, f := range c.Flags {
		ownFlags[i] = classifyFlag(f, scope)
		ownNames[f.Name] = true
	}

	// A command's own flag shadows an inherited global flag of the same
	// name, exactly as real Cobra behaves: a local flag registration wins
	// over a merged-in persistent one, so the global copy is never actually
	// reachable on this command and must be dropped here too (otherwise the
	// Params struct would declare the same field twice).
	var effectiveGlobal []resolvedFlag
	for _, f := range g.globalFlags {
		if !ownNames[f.Name] {
			effectiveGlobal = append(effectiveGlobal, f)
		}
	}

	allFlags = make([]resolvedFlag, 0, len(effectiveGlobal)+len(ownFlags))
	allFlags = append(allFlags, effectiveGlobal...)
	allFlags = append(allFlags, ownFlags...)

	args = make([]resolvedArgument, len(c.Arguments))
	for i, a := range c.Arguments {
		args[i] = classifyArgument(a, scope)
	}

	if err := renderEnumTypes(w, allFlags, args); err != nil {
		return nil, nil, 0, "", err
	}
	if err := renderParamsStruct(w, scope, allFlags, args); err != nil {
		return nil, nil, 0, "", err
	}

	outputType, err = resolveOutputType(c.Output)
	if err != nil {
		return nil, nil, 0, "", fmt.Errorf("command %q: %w", c.Name, err)
	}

	if outputType != "" {
		fmt.Fprintf(w, "\ntype %sHandler func(ctx context.Context, cmd *cobra.Command, io IO, p %sParams) (%s, error)\n", scope, scope, outputType)
	} else {
		fmt.Fprintf(w, "\ntype %sHandler func(ctx context.Context, cmd *cobra.Command, io IO, p %sParams) error\n", scope, scope)
	}

	return allFlags, args, len(effectiveGlobal), outputType, nil
}

func (g *generator) renderLeaf(w *strings.Builder, c spec.Command, topLevel bool) (nodeResult, error) {
	if c.OperationID == "" {
		return nodeResult{}, fmt.Errorf("command %q: leaf commands (no nested commands) must declare operationId", c.Name)
	}
	scope := pascalCase(c.OperationID)

	allFlags, args, effectiveGlobalCount, outputType, err := g.ownFlagsAndArgs(w, c, scope)
	if err != nil {
		return nodeResult{}, err
	}

	funcName, returnType := ctorSignature(scope, topLevel)
	paramVar := camelCase(scope)

	fmt.Fprintf(w, "\nfunc %s(%s %sHandler) %s {\n", funcName, paramVar, scope, returnType)
	renderLeafBody(w, c, scope, allFlags, args, effectiveGlobalCount, paramVar, returnType, nil, outputType)
	w.WriteString("}\n")

	return nodeResult{
		CtorExpr: fmt.Sprintf("%s(%s)", funcName, paramVar),
		Params:   []handlerParam{{VarName: paramVar, TypeName: scope + "Handler"}},
	}, nil
}

// renderHybrid handles a command that is both runnable itself and a parent
// of subcommands. Its own Params/Handler are named from its operationId
// (like a leaf); its constructor and exported Command type are named from
// its position in the tree (like a group), since that's how its parent
// addresses it.
func (g *generator) renderHybrid(w *strings.Builder, c spec.Command, topLevel bool, namePrefix string) (nodeResult, error) {
	scope := pascalCase(c.OperationID)
	groupName := pascalCase(c.Name)
	fullName := groupName
	if !topLevel {
		fullName = namePrefix + groupName
	}
	childPrefix := fullName

	var childResults []nodeResult
	for _, child := range c.Commands {
		res, err := g.renderNode(w, child, false, childPrefix)
		if err != nil {
			return nodeResult{}, err
		}
		childResults = append(childResults, res)
	}

	allFlags, args, effectiveGlobalCount, outputType, err := g.ownFlagsAndArgs(w, c, scope)
	if err != nil {
		return nodeResult{}, err
	}

	selfParam := handlerParam{VarName: camelCase(scope), TypeName: scope + "Handler"}
	params := []handlerParam{selfParam}
	for _, r := range childResults {
		params = append(params, r.Params...)
	}

	funcName, returnType := ctorSignature(fullName, topLevel)

	sig := make([]string, len(params))
	for i, p := range params {
		sig[i] = fmt.Sprintf("%s %s", p.VarName, p.TypeName)
	}
	fmt.Fprintf(w, "\nfunc %s(\n\t%s,\n) %s {\n", funcName, strings.Join(sig, ",\n\t"), returnType)
	renderLeafBody(w, c, scope, allFlags, args, effectiveGlobalCount, selfParam.VarName, returnType, childResults, outputType)
	w.WriteString("}\n")

	callArgs := make([]string, len(params))
	for i, p := range params {
		callArgs[i] = p.VarName
	}
	return nodeResult{
		CtorExpr: fmt.Sprintf("%s(\n\t\t%s,\n\t)", funcName, strings.Join(callArgs, ",\n\t\t")),
		Params:   params,
	}, nil
}

// renderGroup renders a pure group (no operationId): a parent whose entire
// job is to assemble its children under one Use string, with no RunE of its
// own. presetChildren lets a caller that already rendered children (unused
// today, kept for symmetry with renderHybrid) skip re-rendering them.
func (g *generator) renderGroup(w *strings.Builder, c spec.Command, topLevel bool, namePrefix string, presetChildren []nodeResult) (nodeResult, error) {
	groupName := pascalCase(c.Name)
	fullName := groupName
	if !topLevel {
		fullName = namePrefix + groupName
	}
	childPrefix := fullName

	childResults := presetChildren
	if childResults == nil {
		for _, child := range c.Commands {
			res, err := g.renderNode(w, child, false, childPrefix)
			if err != nil {
				return nodeResult{}, err
			}
			childResults = append(childResults, res)
		}
	}

	var params []handlerParam
	for _, r := range childResults {
		params = append(params, r.Params...)
	}

	funcName, returnType := ctorSignature(fullName, topLevel)

	data := groupTemplateData{
		Function: funcName, ReturnType: returnType, Parameters: params,
		Use: goStringLiteral(c.Name), Children: childResults,
		Return: castTo(returnType, "cmd"),
	}
	if c.Description != "" {
		data.Short = goStringLiteral(c.Description)
	}
	if c.LongDescription != "" {
		data.Long = goRawOrQuoted(c.LongDescription)
	}
	if len(c.Aliases) > 0 {
		data.Aliases = goStringSliceLiteral(c.Aliases)
	}
	source, err := executeTemplate("group", data)
	if err != nil {
		return nodeResult{}, err
	}
	w.WriteString(source)

	callArgs := make([]string, len(params))
	for i, p := range params {
		callArgs[i] = p.VarName
	}
	return nodeResult{
		CtorExpr: fmt.Sprintf("%s(\n\t\t%s,\n\t)", funcName, strings.Join(callArgs, ",\n\t\t")),
		Params:   params,
	}, nil
}

type groupTemplateData struct {
	Function   string
	ReturnType string
	Parameters []handlerParam
	Use        string
	Short      string
	Long       string
	Aliases    string
	Children   []nodeResult
	Return     string
}

// ctorSignature returns the constructor function name and return type for a
// node. Top-level nodes get an exported constructor and a named Command type
// (so distinct top-level commands are distinct DI types, matching how the
// hand-written CLIs already distinguish sibling command groups); nested
// nodes get an unexported constructor returning a plain *cobra.Command.
func ctorSignature(scopeOrFullName string, topLevel bool) (funcName, returnType string) {
	if topLevel {
		return "New" + scopeOrFullName + "Command", scopeOrFullName + "Command"
	}
	return "new" + scopeOrFullName + "Command", "*cobra.Command"
}

func castTo(returnType, expr string) string {
	if returnType == "*cobra.Command" {
		return expr
	}
	return fmt.Sprintf("%s(%s)", returnType, expr)
}

func renderEnumTypes(w *strings.Builder, flags []resolvedFlag, args []resolvedArgument) error {
	for _, f := range flags {
		if f.Kind == kindEnum {
			if err := renderEnumType(w, f.EnumType, f.Choices); err != nil {
				return err
			}
		}
	}
	for _, a := range args {
		if a.Kind == kindEnum {
			if err := renderEnumType(w, a.EnumType, a.Choices); err != nil {
				return err
			}
		}
	}
	return nil
}

func renderEnumType(w *strings.Builder, typeName string, choices []string) error {
	data := enumTemplateData{Name: typeName}
	for _, choice := range choices {
		data.Choices = append(data.Choices, enumChoiceTemplateData{
			Constant: typeName + pascalCase(choice),
			Value:    goStringLiteral(choice),
		})
	}
	source, err := executeTemplate("enum", data)
	if err != nil {
		return err
	}
	w.WriteString("\n" + source)
	return nil
}

func renderParamsStruct(w *strings.Builder, scope string, flags []resolvedFlag, args []resolvedArgument) error {
	data := paramsTemplateData{Name: scope}
	for _, f := range flags {
		goType := f.Kind.goType(f.EnumType)
		if f.Kind == kindStringArray || f.Kind == kindStringSlice {
			goType = "[]string"
		}
		data.Fields = append(data.Fields, fieldTemplateData{Name: f.FieldName, Type: goType, TrackChanged: f.TrackChanged})
	}
	for _, a := range args {
		goType := a.Kind.goType(a.EnumType)
		if a.Variadic {
			goType = "[]string"
		}
		data.Fields = append(data.Fields, fieldTemplateData{Name: a.FieldName, Type: goType})
	}
	source, err := executeTemplate("params", data)
	if err != nil {
		return err
	}
	w.WriteString("\n" + source)
	return nil
}

type enumTemplateData struct {
	Name    string
	Choices []enumChoiceTemplateData
}

type enumChoiceTemplateData struct {
	Constant string
	Value    string
}

type paramsTemplateData struct {
	Name   string
	Fields []fieldTemplateData
}

type fieldTemplateData struct {
	Name         string
	Type         string
	TrackChanged bool
}

func goRawOrQuoted(s string) string {
	if strings.Contains(s, "`") {
		return goStringLiteral(s)
	}
	return "`" + s + "`"
}

func goStringSliceLiteral(ss []string) string {
	quoted := make([]string, len(ss))
	for i, s := range ss {
		quoted[i] = goStringLiteral(s)
	}
	return "[]string{" + strings.Join(quoted, ", ") + "}"
}
