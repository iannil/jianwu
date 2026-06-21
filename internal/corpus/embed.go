package corpus

import "embed"

//go:embed builtin/*.json
var builtinFS embed.FS
