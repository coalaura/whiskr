package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/coalaura/plain"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/coalaura/whiskr/internal/desktop"
	"github.com/coalaura/whiskr/internal/open"
	"github.com/coalaura/whiskr/internal/paths"
)

var Version = "dev"

var (
	path     paths.Paths
	prompts  []Prompt
	env      *Environment
	settings *Settings

	log = plain.New(plain.WithDate(plain.RFC3339Local))
)

func main() {
	var err error

	log.Println("Loading paths...")

	path, err = paths.ResolvePaths()
	log.MustFail(err)

	log.Println("Ensuring config...")

	created, err := EnsureEnv()
	log.MustFail(err)

	if created && desktop.IsDesktop {
		log.Printf("Created %s; opening it for setup\n", path.Config)

		err = open.OpenFile(path.Config)
		log.MustFail(err)

		return
	}

	log.Println("Loading prompts...")

	prompts, err = LoadPrompts()
	log.MustFail(err)

	log.Println("Loading environment...")

	env, err = LoadEnv()
	if err != nil && desktop.IsDesktop {
		log.Warnf("Unable to load config: %v; opening %s\n", err, path.Config)

		err = open.OpenFile(path.Config)
		if err == nil {
			return
		} else {
			log.Warnf("Unable to open config: %v\n", err)
		}
	}

	log.MustFail(err)

	log.Println("Loading settings...")

	settings, err = LoadSettings()
	log.MustFail(err)

	defer settings.Store()

	err = StartModelUpdateLoop()
	log.MustFail(err)

	tokenizer, err := LoadTokenizer(TikTokenSource)
	log.MustFail(err)

	log.Println("Calculating overhead...")

	for i, p := range prompts {
		prompts[i].Tokens = tokenizer.CountTokens(p.Text)
	}

	searchToolsJson, _ := json.Marshal(GetSearchTools())

	overhead := map[string]any{
		"files":    tokenizer.CountTokens(InternalFilesPrompt),
		"no_files": tokenizer.CountTokens(InternalNoFilesPrompt),
		"search":   tokenizer.CountTokens(string(searchToolsJson)),
	}

	log.Println("Preparing router...")
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(log.Middleware())

	r.Handle("/*", frontend(env.Debug))

	r.Get("/-/data", func(w http.ResponseWriter, r *http.Request) {
		modelMx.RLock()
		defer modelMx.RUnlock()

		RespondJson(w, http.StatusOK, map[string]any{
			"authenticated": IsAuthenticated(r),
			"config": map[string]any{
				"auth":    env.Authentication.Enabled,
				"search":  env.Tokens.Tavily != "",
				"motion":  env.UI.ReducedMotion,
				"images":  env.Models.ImageGeneration,
				"tts":     env.Models.TextToSpeech,
				"title":   env.Models.TitleModel != "-",
				"proxies": ProxyNames(),
			},
			"overhead":     overhead,
			"models":       ModelList,
			"audio_models": AudioList,
			"prompts":      prompts,
			"version":      Version,
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
		gr.Post("/-/image", HandleImage)
		gr.Post("/-/tts", HandleTTS)

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

	if desktop.IsDesktop {
		desktop.RunDesktop(fmt.Sprintf("http://localhost%s/", addr), env.Debug)
	} else {
		log.WaitForInterrupt()
	}

	log.Warnln("Shutting down...")

	server.Close()
}
