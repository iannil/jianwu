package expand

import "embed"

//go:embed prompt/*.tmpl
var promptFS embed.FS

func loadTemplate(name string) ([]byte, error) {
	return promptFS.ReadFile("prompt/" + name + ".md.tmpl")
}
