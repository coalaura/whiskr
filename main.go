package main

import (
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/coalaura/logger"
	adapter "github.com/coalaura/logger/http"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var Version = "dev"

var log = logger.New().DetectTerminal().WithOptions(logger.Options{
	NoLevel: true,
})

func main() {
	icons, err := LoadIcons()
	log.MustPanic(err)

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
			"authentication": env.Authentication.Enabled,
			"authenticated":  IsAuthenticated(r),
			"search":         env.Tokens.Exa != "",
			"icons":          icons,
			"models":         models,
			"prompts":        Prompts,
			"version":        Version,
		})
	})

	r.Post("/-/auth", HandleAuthentication)

	r.Group(func(gr chi.Router) {
		gr.Use(Authenticate)

		gr.Get("/-/stats/{id}", HandleStats)
		gr.Post("/-/title", HandleTitle)
		gr.Post("/-/chat", HandleChat)
	})

	log.Info("Listening at http://localhost:3443/")
	http.ListenAndServe(":3443", r)
}

func cache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.ToLower(r.URL.Path)
		ext := filepath.Ext(path)

		if ext == ".png" || ext == ".svg" || ext == ".ttf" || strings.HasSuffix(path, ".min.js") || strings.HasSuffix(path, ".min.css") {
			w.Header().Set("Cache-Control", "public, max-age=3024000, immutable")
		} else if env.Debug {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		}

		next.ServeHTTP(w, r)
	})
}

func LoadIcons() ([]string, error) {
	var icons []string

	directory := filepath.Join("static", "css", "icons")

	err := filepath.Walk(directory, func(path string, info fs.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		if strings.HasSuffix(path, ".svg") {
			rel, err := filepath.Rel(directory, path)
			if err != nil {
				return err
			}

			icons = append(icons, filepath.ToSlash(rel))
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return icons, nil
}
