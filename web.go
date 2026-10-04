// Package crabtown embeds the static viewer.
package crabtown

import (
	"embed"
	"io/fs"
)

//go:embed web
var webFiles embed.FS

// WebFS returns the static viewer files (index.html etc.).
func WebFS() fs.FS {
	sub, err := fs.Sub(webFiles, "web")
	if err != nil {
		panic(err)
	}
	return sub
}
