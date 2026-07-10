package web

import (
	"embed"
	"io/fs"
	"net/http"
)

// files contains the browser UI served by `wataridori serve`.
//
//go:embed index.html styles.css dist
var files embed.FS

// Handler serves the embedded web UI without requiring a separate frontend
// build step. RPC routes are mounted separately by the caller.
func Handler() http.Handler {
	sub, err := fs.Sub(files, ".")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}
