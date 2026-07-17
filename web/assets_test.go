package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, req)
	return rec
}

func TestHandlerServesIndex(t *testing.T) {
	rec := get(t, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Errorf("content-type = %q, want text/html", got)
	}
	// The React app mounts here; without it the page renders blank.
	if !strings.Contains(rec.Body.String(), `id="root"`) {
		t.Error("index.html does not contain the #root mount point")
	}
}

// TestHandlerServesHashedAssets walks the embedded bundle instead of naming
// files, because vite content-hashes every asset on rebuild.
func TestHandlerServesHashedAssets(t *testing.T) {
	assets, err := fs.Glob(files, "dist/assets/*")
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) == 0 {
		t.Fatal("no assets embedded; run 'make web-build'")
	}

	var sawJS, sawCSS bool
	for _, asset := range assets {
		path := strings.TrimPrefix(asset, "dist")
		rec := get(t, path)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", path, rec.Code)
			continue
		}
		contentType := rec.Header().Get("Content-Type")
		switch {
		case strings.HasSuffix(path, ".js"):
			sawJS = true
			if !strings.Contains(contentType, "javascript") {
				t.Errorf("%s: content-type = %q, want javascript", path, contentType)
			}
		case strings.HasSuffix(path, ".css"):
			sawCSS = true
			if !strings.Contains(contentType, "text/css") {
				t.Errorf("%s: content-type = %q, want text/css", path, contentType)
			}
		}
	}
	if !sawJS {
		t.Error("no javascript bundle embedded")
	}
	if !sawCSS {
		t.Error("no stylesheet embedded")
	}
}

// TestHandlerDoesNotServeSource fails if the embed pattern ever widens past
// the build output to the TypeScript sources or node_modules.
func TestHandlerDoesNotServeSource(t *testing.T) {
	for _, path := range []string{"/src/main.tsx", "/package.json", "/vite.config.ts"} {
		if rec := get(t, path); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", path, rec.Code)
		}
	}
}
