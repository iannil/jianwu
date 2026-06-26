package revise

import (
	"strings"
	"text/template"
)

func renderRevise(name string, raw []byte, data any) (string, error) {
	tmpl, err := template.New(name).Parse(string(raw))
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
