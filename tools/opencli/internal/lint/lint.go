// Package lint runs semantic checks over a decoded OpenCLI spec — the
// consistency rules that JSON Schema cannot express (dangling $refs, argument
// ordering, identifier collisions, cross-references between flags and groups).
//
// It assumes the spec is already structurally valid (schema-checked); it walks
// the typed IR and reports problems as spec.Issue values, the same type the
// structural validator emits, so both flow through one reporting path.
package lint

import (
	"fmt"
	"strings"

	"github.com/opencli-dev/opencli/tools/opencli/spec"
)

// Check returns all semantic issues found in s, ordered roughly by document
// position. An empty slice means the spec is semantically consistent.
func Check(s *spec.Spec) []spec.Issue {
	l := &linter{spec: s, opIDs: map[string]string{}}

	l.checkFlagSet(s.Flags, "/flags")
	l.checkFlagCollisions(s.Flags, "/flags")
	l.walk(s.Commands, "/commands")

	return l.issues
}

type linter struct {
	spec   *spec.Spec
	issues []spec.Issue
	opIDs  map[string]string // operationId -> first location seen
}

func (l *linter) add(loc, format string, args ...any) {
	l.issues = append(l.issues, spec.Issue{Location: loc, Message: fmt.Sprintf(format, args...)})
}

func (l *linter) walk(cmds []spec.Command, path string) {
	l.checkSiblingNames(cmds, path)

	for i, c := range cmds {
		cpath := fmt.Sprintf("%s/%d", path, i)

		if c.IsRef() {
			if !l.refResolves(c.Ref, "commands") {
				l.add(cpath+"/$ref", "$ref %q does not resolve to a component", c.Ref)
			}
			continue
		}

		if c.OperationID != "" {
			if prev, ok := l.opIDs[c.OperationID]; ok {
				l.add(cpath+"/operationId", "duplicate operationId %q (also at %s)", c.OperationID, prev)
			} else {
				l.opIDs[c.OperationID] = cpath
			}
		}

		l.checkFlagSet(c.Flags, cpath+"/flags")
		l.checkFlagCollisions(c.Flags, cpath+"/flags")
		l.checkArguments(c.Arguments, cpath+"/arguments")
		l.checkFlagGroups(c, cpath)
		l.checkOutput(c, cpath)

		l.walk(c.Commands, cpath+"/commands")
	}
}

// checkFlagSet validates each flag's references and intra-flag constraints.
func (l *linter) checkFlagSet(flags []spec.Flag, path string) {
	for i, f := range flags {
		fpath := fmt.Sprintf("%s/%d", path, i)
		if f.IsRef() {
			if !l.refResolves(f.Ref, "flags") {
				l.add(fpath+"/$ref", "$ref %q does not resolve to a component", f.Ref)
			}
			continue
		}
		if f.Count && f.Repeatable {
			l.add(fpath, "flag %q sets both count and repeatable, which cannot be combined", f.Name)
		}
	}
}

// checkFlagCollisions reports duplicate long names and short flags within one
// flag set, resolving references to their underlying names.
func (l *linter) checkFlagCollisions(flags []spec.Flag, path string) {
	names := map[string]bool{}
	shorts := map[string]bool{}
	for i, f := range flags {
		fpath := fmt.Sprintf("%s/%d", path, i)
		rf, ok := l.resolveFlag(f)
		if !ok {
			continue // dangling ref already reported
		}
		if rf.Name != "" {
			if names[rf.Name] {
				l.add(fpath, "duplicate flag name %q", rf.Name)
			}
			names[rf.Name] = true
		}
		if rf.Short != "" {
			if shorts[rf.Short] {
				l.add(fpath, "duplicate short flag %q", rf.Short)
			}
			shorts[rf.Short] = true
		}
	}
}

// checkArguments enforces positional ordering: required args must precede
// optional ones, and a variadic arg must be last.
func (l *linter) checkArguments(args []spec.Argument, path string) {
	seenOptional := ""
	for i, a := range args {
		apath := fmt.Sprintf("%s/%d", path, i)
		ra := a
		if a.IsRef() {
			if !l.refResolves(a.Ref, "arguments") {
				l.add(apath+"/$ref", "$ref %q does not resolve to a component", a.Ref)
				continue
			}
			ra, _ = l.resolveArgument(a)
		}

		if ra.IsRequired() && seenOptional != "" {
			l.add(apath, "required argument %q follows optional argument %q", ra.Name, seenOptional)
		}
		if !ra.IsRequired() {
			seenOptional = ra.Name
		}
		if ra.Variadic && i != len(args)-1 {
			l.add(apath, "variadic argument %q must be the last argument", ra.Name)
		}
	}
}

// checkFlagGroups verifies every flag named in a group is declared on the
// command (or globally).
func (l *linter) checkFlagGroups(c spec.Command, cpath string) {
	if len(c.FlagGroups) == 0 {
		return
	}
	available := l.commandFlagNames(c)
	for gi, g := range c.FlagGroups {
		for fi, name := range g.Flags {
			if !available[name] {
				l.add(fmt.Sprintf("%s/flagGroups/%d/flags/%d", cpath, gi, fi),
					"flag group references unknown flag %q", name)
			}
		}
	}
}

// checkOutput verifies output.formatFlag names a real flag on the command.
func (l *linter) checkOutput(c spec.Command, cpath string) {
	if c.Output == nil || c.Output.FormatFlag == "" {
		return
	}
	if !l.commandFlagNames(c)[c.Output.FormatFlag] {
		l.add(cpath+"/output/formatFlag",
			"output.formatFlag references unknown flag %q", c.Output.FormatFlag)
	}
}

// commandFlagNames is the set of flag names available to a command: its own
// declared flags plus inherited global flags, with references resolved.
func (l *linter) commandFlagNames(c spec.Command) map[string]bool {
	names := map[string]bool{}
	for _, set := range [][]spec.Flag{l.spec.Flags, c.Flags} {
		for _, f := range set {
			if rf, ok := l.resolveFlag(f); ok && rf.Name != "" {
				names[rf.Name] = true
			}
		}
	}
	return names
}

// checkSiblingNames reports name/alias collisions among sibling commands.
func (l *linter) checkSiblingNames(cmds []spec.Command, path string) {
	seen := map[string]bool{}
	for i, c := range cmds {
		rc := c
		if c.IsRef() {
			rc, _ = l.resolveCommand(c)
		}
		cpath := fmt.Sprintf("%s/%d", path, i)
		for _, name := range append([]string{rc.Name}, rc.Aliases...) {
			if name == "" {
				continue
			}
			if seen[name] {
				l.add(cpath, "command name or alias %q collides with a sibling", name)
			}
			seen[name] = true
		}
	}
}

// --- reference resolution helpers ---

// componentRef splits "#/components/<kind>/<name>" into its kind and name.
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

// refResolves reports whether ref points to an existing component of wantKind.
func (l *linter) refResolves(ref, wantKind string) bool {
	kind, name, ok := componentRef(ref)
	if !ok || kind != wantKind || l.spec.Components == nil {
		return false
	}
	switch kind {
	case "flags":
		_, ok := l.spec.Components.Flags[name]
		return ok
	case "arguments":
		_, ok := l.spec.Components.Arguments[name]
		return ok
	case "commands":
		_, ok := l.spec.Components.Commands[name]
		return ok
	case "examples":
		_, ok := l.spec.Components.Examples[name]
		return ok
	default:
		return false
	}
}

func (l *linter) resolveFlag(f spec.Flag) (spec.Flag, bool) {
	if !f.IsRef() {
		return f, true
	}
	kind, name, ok := componentRef(f.Ref)
	if !ok || kind != "flags" || l.spec.Components == nil {
		return spec.Flag{}, false
	}
	rf, ok := l.spec.Components.Flags[name]
	return rf, ok
}

func (l *linter) resolveArgument(a spec.Argument) (spec.Argument, bool) {
	if !a.IsRef() {
		return a, true
	}
	kind, name, ok := componentRef(a.Ref)
	if !ok || kind != "arguments" || l.spec.Components == nil {
		return spec.Argument{}, false
	}
	ra, ok := l.spec.Components.Arguments[name]
	return ra, ok
}

func (l *linter) resolveCommand(c spec.Command) (spec.Command, bool) {
	if !c.IsRef() {
		return c, true
	}
	kind, name, ok := componentRef(c.Ref)
	if !ok || kind != "commands" || l.spec.Components == nil {
		return spec.Command{}, false
	}
	rc, ok := l.spec.Components.Commands[name]
	return rc, ok
}
