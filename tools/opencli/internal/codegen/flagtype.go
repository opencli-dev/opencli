package codegen

import (
	"fmt"
	"strconv"

	"github.com/opencli-dev/opencli/tools/opencli/spec"
)

// flagKind classifies how a flag or argument is registered with pflag and
// represented in a generated Params struct. It is derived from the flag's
// declared type/format/repeatable/count, not read back from Go source, so it
// is the single place that decides the mapping.
type flagKind int

const (
	kindString flagKind = iota
	kindUUID            // format: uuid — parsed into uuid.UUID
	kindDuration        // format: duration — native pflag Duration
	kindInt
	kindFloat
	kindBool
	kindCount        // count: true — native pflag Count, always int
	kindEnum         // choices present — named string type with constants
	kindStringArray  // repeatable, no comma-splitting (Cobra StringArrayVar)
	kindStringSlice  // repeatable + splitOnComma (Cobra StringSliceVar)
)

// resolvedFlag is a flag together with everything codegen needs to emit it:
// its Go field name, its kind, and (for enums) the generated type name.
type resolvedFlag struct {
	spec.Flag
	FieldName string
	Kind      flagKind
	EnumType  string // only set when Kind == kindEnum
}

// classifyFlag derives a flag's flagKind from its declared shape. scope is a
// PascalCase prefix (the owning command or "Global") used to name generated
// enum types uniquely.
func classifyFlag(f spec.Flag, scope string) resolvedFlag {
	rf := resolvedFlag{Flag: f, FieldName: pascalCase(f.Name)}

	switch {
	case f.Count:
		rf.Kind = kindCount
	case f.Repeatable:
		if f.SplitOnComma {
			rf.Kind = kindStringSlice
		} else {
			rf.Kind = kindStringArray
		}
	case len(f.Choices) > 0:
		rf.Kind = kindEnum
		rf.EnumType = scope + rf.FieldName
	case f.Format == "uuid":
		rf.Kind = kindUUID
	case f.Format == "duration":
		rf.Kind = kindDuration
	default:
		switch f.Type {
		case "integer":
			rf.Kind = kindInt
		case "number":
			rf.Kind = kindFloat
		case "boolean":
			rf.Kind = kindBool
		default: // string, file, path, or unset
			rf.Kind = kindString
		}
	}
	return rf
}

// resolvedArgument mirrors resolvedFlag for positional arguments, which share
// the same type vocabulary (string/uuid/duration/choices) but never support
// count/repeatable-with-splitting — a variadic argument is always a plain
// string slice.
type resolvedArgument struct {
	spec.Argument
	FieldName string
	Kind      flagKind
	EnumType  string
}

func classifyArgument(a spec.Argument, scope string) resolvedArgument {
	ra := resolvedArgument{Argument: a, FieldName: pascalCase(a.Name)}

	switch {
	case len(a.Choices) > 0:
		ra.Kind = kindEnum
		ra.EnumType = scope + ra.FieldName
	case a.Format == "uuid":
		ra.Kind = kindUUID
	case a.Format == "duration":
		ra.Kind = kindDuration
	default:
		switch a.Type {
		case "integer":
			ra.Kind = kindInt
		case "number":
			ra.Kind = kindFloat
		case "boolean":
			ra.Kind = kindBool
		default:
			ra.Kind = kindString
		}
	}
	return ra
}

// goType is the Go type used for a Params struct field of this kind. Variadic
// arguments and string-array/slice flags are handled by the caller, which
// wraps this in []T.
func (k flagKind) goType(enumType string) string {
	switch k {
	case kindUUID:
		return "uuid.UUID"
	case kindDuration:
		return "time.Duration"
	case kindInt, kindCount:
		return "int"
	case kindFloat:
		return "float64"
	case kindBool:
		return "bool"
	case kindEnum:
		return enumType
	default:
		return "string"
	}
}

// rawGoType is the type used for the intermediate pflag-bound variable before
// any post-parse conversion (uuid/enum). For kinds with no post-parse step
// this is identical to goType.
func (k flagKind) rawGoType() string {
	switch k {
	case kindUUID, kindEnum:
		return "string"
	case kindDuration:
		return "time.Duration"
	case kindInt, kindCount:
		return "int"
	case kindFloat:
		return "float64"
	case kindBool:
		return "bool"
	case kindStringArray, kindStringSlice:
		return "[]string"
	default:
		return "string"
	}
}

// pflagVarFunc returns the pflag *Var method name (without the VarP/Var
// suffix decision, handled by the caller based on whether a shorthand is
// set) for registering a flag of this kind.
func (k flagKind) pflagVarFunc() string {
	switch k {
	case kindUUID, kindEnum:
		return "StringVar"
	case kindDuration:
		return "DurationVar"
	case kindInt:
		return "IntVar"
	case kindCount:
		return "CountVar"
	case kindFloat:
		return "Float64Var"
	case kindBool:
		return "BoolVar"
	case kindStringArray:
		return "StringArrayVar"
	case kindStringSlice:
		return "StringSliceVar"
	default:
		return "StringVar"
	}
}

// pflagPlainFunc is pflagVarFunc without a bound Go variable — used to
// register a global flag on the root command, where nothing local needs to
// read it back (every command that cares reads it via cmd.Flags().GetXxx).
func (k flagKind) pflagPlainFunc() string {
	switch k {
	case kindUUID, kindEnum:
		return "String"
	case kindDuration:
		return "Duration"
	case kindInt:
		return "Int"
	case kindCount:
		return "Count"
	case kindFloat:
		return "Float64"
	case kindBool:
		return "Bool"
	case kindStringArray:
		return "StringArray"
	case kindStringSlice:
		return "StringSlice"
	default:
		return "String"
	}
}

// pflagGetFunc returns the pflag FlagSet getter used to read a flag that was
// registered on an ancestor command (a global/persistent flag), by name,
// rather than through a locally bound variable.
func (k flagKind) pflagGetFunc() string {
	switch k {
	case kindUUID, kindEnum:
		return "GetString"
	case kindDuration:
		return "GetDuration"
	case kindInt:
		return "GetInt"
	case kindCount:
		return "GetCount"
	case kindFloat:
		return "GetFloat64"
	case kindBool:
		return "GetBool"
	case kindStringArray:
		return "GetStringArray"
	case kindStringSlice:
		return "GetStringSlice"
	default:
		return "GetString"
	}
}

// goDefaultLiteral renders a flag/argument's declared default as a Go source
// literal of the raw pflag-bound type. A nil default renders as the zero
// value for that type.
func goDefaultLiteral(k flagKind, def any) string {
	if def == nil {
		switch k.rawGoType() {
		case "string":
			return `""`
		case "int":
			return "0"
		case "float64":
			return "0"
		case "bool":
			return "false"
		case "time.Duration":
			return "0"
		default:
			return "nil"
		}
	}

	switch k {
	case kindDuration:
		if s, ok := def.(string); ok {
			return fmt.Sprintf("mustParseDuration(%s)", goStringLiteral(s))
		}
		return "0"
	}

	switch v := def.(type) {
	case string:
		return goStringLiteral(v)
	case bool:
		return strconv.FormatBool(v)
	case float64:
		switch k.rawGoType() {
		case "int":
			return strconv.Itoa(int(v))
		default:
			return strconv.FormatFloat(v, 'g', -1, 64)
		}
	default:
		return fmt.Sprintf("%v", v)
	}
}
