package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web/dist
var staticFiles embed.FS

// spaFileServer returns an http.Handler that serves the embedded Vite build.
// Unknown paths fall back to index.html so that client-side routing works.
func spaFileServer() http.Handler {
	dist, err := fs.Sub(staticFiles, "web/dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to open the requested file. If it doesn't exist, serve index.html.
		f, err := dist.Open(r.URL.Path[1:]) // strip leading "/"
		if err != nil {
			r.URL.Path = "/"
		} else {
			f.Close()
		}
		fileServer.ServeHTTP(w, r)
	})
}
