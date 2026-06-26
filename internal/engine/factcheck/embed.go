package factcheck

import "embed"

//go:embed prompt/*.tmpl
var promptFS embed.FS

func loadSystem() ([]byte, error) { return promptFS.ReadFile("prompt/system.md.tmpl") }
func loadUser() ([]byte, error)   { return promptFS.ReadFile("prompt/user.md.tmpl") }
