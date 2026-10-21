package console

import "embed"

// FS holds the embedded Console static assets.
//
//go:embed index.html app.js styles.css
var FS embed.FS
