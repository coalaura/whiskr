//go:build !desktop

package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

func frontend() http.Handler {
	if !env.Debug {
		server := http.FileServer(http.Dir("./public"))

		return cache(server)
	}

	target, _ := url.Parse("http://localhost:3000")
	proxy := httputil.NewSingleHostReverseProxy(target)

	log.Println("Proxying frontend requests to Rsbuild (:3000)")

	return cache(proxy)
}
