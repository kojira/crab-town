package crabtown

import (
	"io/fs"
	"testing"
)

// The viewer is split into several files; all of them must be embedded.
func TestWebFSEmbedsViewer(t *testing.T) {
	for _, name := range []string{"index.html", "sprites.js", "furniture.js", "floor.js", "props.js", "town.js", "render.js", "ws.js", "nostr.html", "nostr.js", "config.js", "viewport.js", "chatlog.js", "mobile.js", "avatar.js", "nostrtools.js"} {
		b, err := fs.ReadFile(WebFS(), name)
		if err != nil || len(b) == 0 {
			t.Fatalf("%s not embedded: %v", name, err)
		}
	}
}
