package main

import (
	"net/http"

	"github.com/coalaura/logger"
	adapter "github.com/coalaura/logger/http"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var log = logger.New().DetectTerminal().WithOptions(logger.Options{
	NoLevel: true,
})

func main() {
	models, err := LoadModels()
	log.MustPanic(err)

	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(adapter.Middleware(log))

	fs := http.FileServer(http.Dir("./static"))
	r.Handle("/*", http.StripPrefix("/", fs))

	r.Get("/-/models", func(w http.ResponseWriter, r *http.Request) {
		RespondJson(w, http.StatusOK, models)
	})

	r.Post("/-/chat", HandleChat)

	log.Debug("Listening at http://localhost:3443/")
	http.ListenAndServe(":3443", r)
}
