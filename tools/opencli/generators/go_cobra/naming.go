package go_cobra

import "strings"

// pascalCase converts a kebab-case, snake_case, SCREAMING_SNAKE_CASE, or
// already-camelCase identifier (as used for command names, flag names,
// argument names, and operationIds) into an exported Go identifier. It only
// splits on '-', '_', and ' ', so a camelCase operationId like
// "deviceCreate" (no delimiters) becomes "DeviceCreate" by uppercasing its
// first rune alone, preserving the internal capital. A SCREAMING token like
// "DEVICE" or "ID" (from an argument name like "DEVICE_ID") is title-cased
// instead — otherwise concatenating already-uppercase tokens would produce
// an unreadable run like "DEVICEID".
func pascalCase(s string) string {
	parts := splitIdent(s)
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		if p == strings.ToUpper(p) && len(p) > 1 {
			b.WriteString(strings.ToUpper(p[:1]))
			b.WriteString(strings.ToLower(p[1:]))
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(p[1:])
	}
	return b.String()
}

// camelCase is pascalCase with the first rune lower-cased, for unexported
// identifiers (local variables, unexported helper functions).
func camelCase(s string) string {
	p := pascalCase(s)
	if p == "" {
		return p
	}
	return strings.ToLower(p[:1]) + p[1:]
}

func splitIdent(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
}

// placeholder renders the usage-line placeholder for an argument: its
// explicit Placeholder if set, otherwise a lowercase-kebab form of its name
// (ARGUMENT_NAME -> argument-name), wrapped in <> when required or [] when
// optional, with a trailing "..." after the bracket when variadic (e.g.
// "<slug>...", not "<slug...>").
func placeholder(name, custom string, required, variadic bool) string {
	text := custom
	if text == "" {
		text = strings.ToLower(strings.ReplaceAll(name, "_", "-"))
	}
	wrapped := "[" + text + "]"
	if required {
		wrapped = "<" + text + ">"
	}
	if variadic {
		wrapped += "..."
	}
	return wrapped
}

// goStringLiteral renders s as a double-quoted Go string literal.
func goStringLiteral(s string) string {
	return quoteGo(s)
}

func quoteGo(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
