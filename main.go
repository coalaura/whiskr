package main

import (
	_ "embed"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/coalaura/plain"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var Version = "dev"

var (
	env      *Environment
	settings *Settings

	log = plain.New(plain.WithDate(plain.RFC3339Local))
)

func main() {
	var err error

	log.Println("Loading environment...")

	env, err = LoadEnv()
	log.MustFail(err)

	log.Println("Loading settings...")

	settings, err = LoadSettings()
	log.MustFail(err)

	defer settings.Store()

	err = StartModelUpdateLoop()
	log.MustFail(err)

	tokenizer, err := LoadTokenizer(TikTokenSource)
	log.MustFail(err)

	log.Println("Preparing router...")
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(log.Middleware())

	r.Handle("/*", cache(frontend()))

	r.Get("/-/data", func(w http.ResponseWriter, r *http.Request) {
		modelMx.RLock()
		defer modelMx.RUnlock()

		RespondJson(w, http.StatusOK, map[string]any{
			"authenticated": IsAuthenticated(r),
			"config": map[string]any{
				"auth":   env.Authentication.Enabled,
				"search": env.Tokens.Exa != "",
				"motion": env.UI.ReducedMotion,
				"images": env.Models.ImageGeneration,
			},
			"models":  ModelList,
			"prompts": Prompts,
			"version": Version,
		})
	})

	r.Get("/-/settings", func(w http.ResponseWriter, r *http.Request) {
		user := GetAuthenticatedUser(r)
		if user == nil {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		RespondJson(w, http.StatusOK, settings.Serialize(user.Username))
	})

	r.Post("/-/auth", HandleAuthentication)

	r.Group(func(gr chi.Router) {
		gr.Use(Authenticate)

		gr.Get("/-/usage", HandleUsage)
		gr.Post("/-/title", HandleTitle)

		gr.Post("/-/chat", HandleChat)
		gr.Post("/-/dump", HandleDump)

		gr.Post("/-/tokenize", HandleTokenize(tokenizer))
		gr.Post("/-/preview", HandlePreview)

		gr.Patch("/-/settings/{setting}", HandleUserSetting)
	})

	addr := env.Addr()

	server := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		log.Printf("Listening at http://localhost%s/\n", addr)

		err = server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Warnln(err)
		}
	}()

	log.WaitForInterrupt()

	log.Warnln("Shutting down...")

	server.Close()
}

func cache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if env.Debug {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

			next.ServeHTTP(w, r)

			return
		}

		path := strings.ToLower(r.URL.Path)
		ext := filepath.Ext(path)

		if ext == ".png" || ext == ".svg" || ext == ".ttf" || strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".css") {
			w.Header().Set("Cache-Control", "public, max-age=3024000, immutable")
		}

		next.ServeHTTP(w, r)
	})
}

func frontend() http.Handler {
	if !env.Debug {
		return http.FileServer(http.Dir("./public"))
	}

	target, _ := url.Parse("http://localhost:3000")
	proxy := httputil.NewSingleHostReverseProxy(target)

	log.Println("Proxying frontend requests to Rsbuild (:3000)")

	return proxy
}
