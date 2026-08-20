//go:build desktop && release

package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed public
var frontendFS embed.FS

func frontend(bool) http.Handler {
	pub, err := fs.Sub(frontendFS, "public")
	if err != nil {
		panic(err)
	}

	return cache(http.FileServer(http.FS(pub)))
}
