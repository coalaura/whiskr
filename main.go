package main

import (
	"errors"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/coalaura/logger"
	adapter "github.com/coalaura/logger/http"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const Version = "dev"

var log = logger.New().DetectTerminal().WithOptions(logger.Options{
	NoLevel: true,
})

func main() {
	log.Info("Loading models...")

	models, err := LoadModels()
	log.MustPanic(err)

	log.Info("Preparing router...")
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(adapter.Middleware(log))

	fs := http.FileServer(http.Dir("./static"))
	r.Handle("/*", cache(http.StripPrefix("/", fs)))

	r.Get("/-/data", func(w http.ResponseWriter, r *http.Request) {
		RespondJson(w, http.StatusOK, map[string]any{
			"version": Version,
			"models":  models,
		})
	})

	r.Get("/-/stats/{id}", HandleStats)
	r.Post("/-/chat", HandleChat)

	if !NoOpen {
		time.AfterFunc(500*time.Millisecond, func() {
			log.Info("Opening browser...")

			err := open("http://localhost:3443/")
			if err != nil {
				log.WarningE(err)
			}
		})
	}

	log.Info("Listening at http://localhost:3443/")
	http.ListenAndServe(":3443", r)
}

func cache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.ToLower(r.URL.Path)
		ext := filepath.Ext(path)

		if ext == ".svg" || ext == ".ttf" || strings.HasSuffix(path, ".min.js") || strings.HasSuffix(path, ".min.css") {
			w.Header().Set("Cache-Control", "public, max-age=3024000, immutable")
		}

		next.ServeHTTP(w, r)
	})
}

func open(url string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	}

	return errors.New("unsupported platform")
}
