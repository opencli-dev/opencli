package go_cobra

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed templates/*.tmpl
var templateFiles embed.FS

var sourceTemplates = template.Must(template.New("go-cobra").ParseFS(templateFiles, "templates/*.tmpl"))

func executeTemplate(name string, data any) (string, error) {
	var out bytes.Buffer
	if err := sourceTemplates.ExecuteTemplate(&out, name, data); err != nil {
		return "", fmt.Errorf("go-cobra: execute template %q: %w", name, err)
	}
	return out.String(), nil
}
