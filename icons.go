package main

import (
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
)

var iconPathRgx = regexp.MustCompile(`(?im)^\w+.[a-z]+$`)

func HandleIcon(w http.ResponseWriter, r *http.Request) {
	path := chi.URLParam(r, "path")
	if !isValidIconPath(path) {
		w.WriteHeader(http.StatusBadRequest)

		return
	}

	resp, err := http.Get("https://openrouter.ai/images/icons/" + path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		w.WriteHeader(resp.StatusCode)

		return
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/png"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3024000, immutable")

	io.Copy(w, resp.Body)
}

func isValidIconPath(path string) bool {
	if len(path) < 3 {
		return false
	}

	if !iconPathRgx.MatchString(path) {
		return false
	}

	ext := filepath.Ext(path)

	switch strings.ToLower(ext) {
	case ".png", ".webp", ".jpg", ".jpeg", ".svg":
		return true
	}

	return false
}
