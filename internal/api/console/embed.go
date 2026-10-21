package console

import "embed"

// FS holds the embedded Console static assets.
//
//go:embed index.html
var FS embed.FS
