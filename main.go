package main

import (
	_ "embed"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/coalaura/plain"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var Version = "dev"

var log = plain.New(plain.WithDate(plain.RFC3339Local))

func main() {
	icons, err := LoadIcons()
	log.MustFail(err)

	err = StartModelUpdateLoop()
	log.MustFail(err)

	tokenizer, err := LoadTokenizer(TikTokenSource)
	log.MustFail(err)

	log.Println("Preparing router...")
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(log.Middleware())

	fs := http.FileServer(http.Dir("./static"))
	r.Handle("/*", cache(http.StripPrefix("/", fs)))

	r.Get("/-/data", func(w http.ResponseWriter, r *http.Request) {
		modelMx.RLock()
		defer modelMx.RUnlock()

		RespondJson(w, http.StatusOK, map[string]any{
			"authenticated": IsAuthenticated(r),
			"config": map[string]any{
				"auth":   env.Authentication.Enabled,
				"search": env.Tokens.Exa != "",
				"motion": env.UI.ReducedMotion,
			},
			"icons":   icons,
			"models":  ModelList,
			"prompts": Prompts,
			"version": Version,
		})
	})

	r.Post("/-/auth", HandleAuthentication)

	r.Group(func(gr chi.Router) {
		gr.Use(Authenticate)

		gr.Get("/-/stats/{id}", HandleStats)
		gr.Post("/-/title", HandleTitle)

		gr.Post("/-/chat", HandleChat)
		gr.Post("/-/dump", HandleDump)

		gr.Post("/-/tokenize", HandleTokenize(tokenizer))
		gr.Post("/-/preview", HandlePreview)
	})

	addr := env.Addr()

	log.Printf("Listening at http://localhost%s/\n", addr)
	http.ListenAndServe(addr, r)
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
	log.Println("Loading icons...")

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

	log.Printf("Loaded %d icons\n", len(icons))

	return icons, nil
}
