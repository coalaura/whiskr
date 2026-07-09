package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strings"

	"github.com/coalaura/plain"
)

var log = plain.New(plain.WithDate(plain.RFC3339Local))

func main() {
	log.Println("Loading environment...")

	env, err := LoadEnv()
	log.MustFail(err)

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	token := []byte(env.Server.Token)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := []byte(r.Header.Get("X-Proxy-Auth"))

		if subtle.ConstantTimeCompare(auth, token) != 1 {
			w.WriteHeader(http.StatusUnauthorized)

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
			w.WriteHeader(http.StatusBadRequest)

			log.Warnf("new request: %v\n", err)

			return
		}

		req.Host = "openrouter.ai"

		for key, values := range r.Header {
			if strings.EqualFold(key, "X-Proxy-Auth") || strings.EqualFold(key, "Host") {
				continue
			}

			req.Header[key] = values
		}

		resp, err := client.Do(req)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)

			log.Warnf("send request: %v\n", err)

			return
		}

		defer resp.Body.Close()

		maps.Copy(w.Header(), resp.Header)

		w.WriteHeader(resp.StatusCode)

		flusher, _ := w.(http.Flusher)

		_, err = io.Copy(FlushingWriter{wr: w, fl: flusher}, resp.Body)
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Warnf("proxy copy: %v\n", err)
		}
	})

	addr := env.Addr()

	log.Printf("Listening on %s.\n", addr)

	err = http.ListenAndServe(addr, log.Middleware()(handler))
	log.MustFail(err)
}
