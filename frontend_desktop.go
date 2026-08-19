//go:build desktop

package main

import (
	"embed"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
)

//go:embed public
var frontendFS embed.FS

func frontend() http.Handler {
	if env.Debug {
		target, _ := url.Parse("http://localhost:3000")
		proxy := httputil.NewSingleHostReverseProxy(target)

		log.Println("Proxying frontend requests to Rsbuild (:3000)")

		return cache(proxy)
	}

	pub, err := fs.Sub(frontendFS, "public")
	if err != nil {
		panic(err)
	}

	return cache(http.FileServer(http.FS(pub)))
}
