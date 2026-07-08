package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"flag"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

type FlushingWriter struct {
	wr http.ResponseWriter
	fl http.Flusher
}

func (fw FlushingWriter) Write(p []byte) (int, error) {
	n, err := fw.wr.Write(p)
	if fw.fl != nil {
		fw.fl.Flush()
	}

	return n, err
}

func main() {
	listen := flag.String("listen", ":8443", "listen address")
	auth := flag.String("auth", "", "shared secret required from whiskr (sent as X-Whiskr-Auth)")

	flag.Parse()

	if *auth == "" {
		log.Fatal("whiskr-proxy: -auth is required")
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	handler := http.HandlerFunc(func(wr http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Whiskr-Auth")), []byte(*auth)) == 1 {
			http.Error(wr, "unauthorized", http.StatusUnauthorized)

			return
		}

		target := &url.URL{
			Scheme:   "https",
			Host:     "openrouter.ai",
			Path:     r.URL.Path,
			RawQuery: r.URL.RawQuery,
		}

		req, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), r.Body)
		if err != nil {
			http.Error(wr, err.Error(), http.StatusBadRequest)

			return
		}

		req.Host = "openrouter.ai"

		for key, values := range r.Header {
			if strings.EqualFold(key, "X-Whiskr-Auth") || strings.EqualFold(key, "Host") {
				continue
			}

			req.Header[key] = values
		}

		resp, err := client.Do(req)
		if err != nil {
			http.Error(wr, err.Error(), http.StatusBadGateway)
			return
		}

		defer resp.Body.Close()

		for key, values := range resp.Header {
			wr.Header()[key] = values
		}

		wr.WriteHeader(resp.StatusCode)

		flusher, _ := wr.(http.Flusher)

		_, err = io.Copy(FlushingWriter{wr: wr, fl: flusher}, resp.Body)
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("proxy copy: %v", err)
		}
	})

	log.Printf("whiskr-proxy listening on %s", *listen)
	log.Fatal(http.ListenAndServe(*listen, handler))
}
