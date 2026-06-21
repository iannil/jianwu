package style

import "embed"

//go:embed style-guide.md
var guideFS []byte

//go:embed samples/*.md
var samplesFS embed.FS
