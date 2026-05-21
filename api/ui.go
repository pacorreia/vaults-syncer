package api

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web
var webFiles embed.FS

// ServeUI returns an http.Handler that serves the embedded web frontend.
func ServeUI() http.Handler {
	sub, err := fs.Sub(webFiles, "web")
	if err != nil {
		// Should never happen; fail loudly at startup.
		panic("api: failed to create web sub-FS: " + err.Error())
	}
	return http.FileServer(http.FS(sub))
}
