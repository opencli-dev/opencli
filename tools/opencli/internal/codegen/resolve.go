package codegen

import (
	"fmt"
	"strings"

	"github.com/opencli-dev/opencli/tools/opencli/spec"
)

// resolver expands $ref entries against a spec's components. Callers are
// expected to run validate.Check first, so refs are assumed to already
// resolve (checked by lint) — resolver returns an error only as a defensive
// fallback, e.g. when codegen runs standalone against an unvalidated spec.
type resolver struct {
	components *spec.Components
}

func newResolver(s *spec.Spec) *resolver {
	return &resolver{components: s.Components}
}

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

func (r *resolver) flag(f spec.Flag) (spec.Flag, error) {
	if !f.IsRef() {
		return f, nil
	}
	kind, name, ok := componentRef(f.Ref)
	if !ok || kind != "flags" || r.components == nil {
		return spec.Flag{}, fmt.Errorf("dangling flag $ref %q", f.Ref)
	}
	rf, ok := r.components.Flags[name]
	if !ok {
		return spec.Flag{}, fmt.Errorf("dangling flag $ref %q", f.Ref)
	}
	return rf, nil
}

func (r *resolver) argument(a spec.Argument) (spec.Argument, error) {
	if !a.IsRef() {
		return a, nil
	}
	kind, name, ok := componentRef(a.Ref)
	if !ok || kind != "arguments" || r.components == nil {
		return spec.Argument{}, fmt.Errorf("dangling argument $ref %q", a.Ref)
	}
	ra, ok := r.components.Arguments[name]
	if !ok {
		return spec.Argument{}, fmt.Errorf("dangling argument $ref %q", a.Ref)
	}
	return ra, nil
}

func (r *resolver) command(c spec.Command) (spec.Command, error) {
	if !c.IsRef() {
		return c, nil
	}
	kind, name, ok := componentRef(c.Ref)
	if !ok || kind != "commands" || r.components == nil {
		return spec.Command{}, fmt.Errorf("dangling command $ref %q", c.Ref)
	}
	rc, ok := r.components.Commands[name]
	if !ok {
		return spec.Command{}, fmt.Errorf("dangling command $ref %q", c.Ref)
	}
	return rc, nil
}

// flags resolves every entry of a flag set in order.
func (r *resolver) flags(fs []spec.Flag) ([]spec.Flag, error) {
	out := make([]spec.Flag, 0, len(fs))
	for _, f := range fs {
		rf, err := r.flag(f)
		if err != nil {
			return nil, err
		}
		out = append(out, rf)
	}
	return out, nil
}

// arguments resolves every entry of an argument list in order.
func (r *resolver) arguments(as []spec.Argument) ([]spec.Argument, error) {
	out := make([]spec.Argument, 0, len(as))
	for _, a := range as {
		ra, err := r.argument(a)
		if err != nil {
			return nil, err
		}
		out = append(out, ra)
	}
	return out, nil
}

// commands resolves every entry of a command list in order, recursing into
// each command's own flags/arguments/nested commands.
func (r *resolver) commands(cs []spec.Command) ([]spec.Command, error) {
	out := make([]spec.Command, 0, len(cs))
	for _, c := range cs {
		rc, err := r.command(c)
		if err != nil {
			return nil, err
		}

		flags, err := r.flags(rc.Flags)
		if err != nil {
			return nil, err
		}
		args, err := r.arguments(rc.Arguments)
		if err != nil {
			return nil, err
		}
		children, err := r.commands(rc.Commands)
		if err != nil {
			return nil, err
		}

		rc.Ref = ""
		rc.Flags = flags
		rc.Arguments = args
		rc.Commands = children
		out = append(out, rc)
	}
	return out, nil
}
