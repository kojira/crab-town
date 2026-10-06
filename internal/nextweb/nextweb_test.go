package nextweb_test

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kojira/crab-town/internal/next"
	"github.com/kojira/crab-town/internal/nextweb"
)

// The built viewer is embedded: index.html loads app.js and app.css.
func TestFSEmbedsViewer(t *testing.T) {
	for _, name := range []string{"index.html", "app.js", "app.css"} {
		b, err := fs.ReadFile(nextweb.FS(), name)
		if err != nil || len(b) == 0 {
			t.Fatalf("%s not embedded: %v", name, err)
		}
	}
	b, _ := fs.ReadFile(nextweb.FS(), "index.html")
	for _, want := range []string{`src="app.js"`, `href="app.css"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("index.html does not reference %s", want)
		}
	}
}

// The town-next server serves this viewer at / (not the current town's web/).
func TestServedByTownNext(t *testing.T) {
	srv := httptest.NewServer((&next.Server{Static: nextweb.FS()}).Handler())
	defer srv.Close()
	res, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || !strings.Contains(string(b), "crab-town next") || !strings.Contains(string(b), `id="map"`) {
		t.Fatalf("GET / = %d %q", res.StatusCode, b)
	}
}
