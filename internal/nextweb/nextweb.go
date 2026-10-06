// Package nextweb embeds the town-next viewer (internal/nextweb/static,
// generated from web-next-src/ by scripts/build-next-web.mjs). Only
// cmd/town-next serves it; the current town keeps serving web/.
package nextweb

import (
	"embed"
	"io/fs"
)

//go:embed static
var files embed.FS

// FS returns the viewer files (index.html, app.js, app.css).
func FS() fs.FS {
	sub, err := fs.Sub(files, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
