package rust_clap

import (
	"strings"
	"unicode"
)

var rustKeywords = map[string]bool{
	"as": true, "async": true, "await": true, "break": true, "const": true,
	"continue": true, "crate": true, "dyn": true, "else": true, "enum": true,
	"extern": true, "false": true, "fn": true, "for": true, "if": true,
	"impl": true, "in": true, "let": true, "loop": true, "match": true,
	"mod": true, "move": true, "mut": true, "pub": true, "ref": true,
	"return": true, "self": true, "Self": true, "static": true, "struct": true,
	"super": true, "trait": true, "true": true, "type": true, "union": true,
	"unsafe": true, "use": true, "where": true, "while": true, "abstract": true,
	"become": true, "box": true, "do": true, "final": true, "macro": true,
	"override": true, "priv": true, "typeof": true, "unsized": true,
	"virtual": true, "yield": true, "try": true,
}

func words(value string) []string {
	var out []string
	var word []rune
	flush := func() {
		if len(word) > 0 {
			out = append(out, strings.ToLower(string(word)))
			word = nil
		}
	}
	var previous rune
	for _, current := range value {
		if !unicode.IsLetter(current) && !unicode.IsDigit(current) {
			flush()
			previous = 0
			continue
		}
		if len(word) > 0 && unicode.IsUpper(current) && unicode.IsLower(previous) {
			flush()
		}
		word = append(word, current)
		previous = current
	}
	flush()
	return out
}

func snake(value string) string {
	parts := words(value)
	name := strings.Join(parts, "_")
	if name == "" {
		name = "value"
	}
	if unicode.IsDigit([]rune(name)[0]) {
		name = "_" + name
	}
	if rustKeywords[name] {
		name += "_"
	}
	return name
}

func pascal(value string) string {
	var out strings.Builder
	for _, part := range words(value) {
		runes := []rune(part)
		if len(runes) == 0 {
			continue
		}
		out.WriteRune(unicode.ToUpper(runes[0]))
		out.WriteString(string(runes[1:]))
	}
	name := out.String()
	if name == "" {
		name = "Value"
	}
	if unicode.IsDigit([]rune(name)[0]) {
		name = "Value" + name
	}
	return name
}

func rustString(value string) string {
	for hashes := 0; ; hashes++ {
		marker := strings.Repeat("#", hashes)
		if !strings.Contains(value, "\""+marker) {
			return "r" + marker + "\"" + value + "\"" + marker
		}
	}
}
