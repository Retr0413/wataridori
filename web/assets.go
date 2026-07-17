package web

import (
	"embed"
	"io/fs"
	"net/http"
)

// files contains the built browser UI served by `wataridori serve`. The
// contents come from `make web-build` (vite) and are committed, so `go build`
// and goreleaser never need node installed.
//
//go:embed all:dist
var files embed.FS

// Handler serves the embedded web UI. Vite content-hashes the asset file
// names, so index.html is the only stable entry point. RPC routes are mounted
// separately by the caller.
func Handler() http.Handler {
	sub, err := fs.Sub(files, "dist")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}
