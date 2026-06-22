package outline

import "embed"

//go:embed prompt/*.tmpl
var promptFS embed.FS

// loadSystem returns the parsed system.md.tmpl.
func loadSystem() ([]byte, error) {
	return promptFS.ReadFile("prompt/system.md.tmpl")
}

// loadUser returns the parsed user.md.tmpl.
func loadUser() ([]byte, error) {
	return promptFS.ReadFile("prompt/user.md.tmpl")
}
