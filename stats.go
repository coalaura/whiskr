package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/revrost/go-openrouter"
)

type Statistics struct {
	Provider     *string `json:"provider,omitempty"`
	Model        string  `json:"model"`
	Cost         float64 `json:"cost"`
	TTFT         int     `json:"ttft"`
	Time         int     `json:"time"`
	InputTokens  int     `json:"input"`
	OutputTokens int     `json:"output"`
}

func HandleStats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if id == "" || !strings.HasPrefix(id, "gen-") {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": "invalid id",
		})

		return
	}

	ctx := r.Context()

	var (
		attempt    int
		generation openrouter.Generation
		err        error

		backoff = time.Second
	)

	for attempt < 4 {
		generation, err = OpenRouterGetGeneration(ctx, id)
		if err == nil {
			break
		}

		log.Println(err)

		attempt++

		time.Sleep(backoff)

		backoff = min(4*time.Second, backoff*2)
	}

	if err != nil {
		RespondJson(w, http.StatusInternalServerError, map[string]any{
			"error": err.Error(),
		})

		return
	}

	statistics := Statistics{
		Provider: generation.ProviderName,
		Model:    generation.Model,
		Cost:     generation.TotalCost,
		TTFT:     Nullable(generation.Latency, 0),
		Time:     Nullable(generation.GenerationTime, 0),
	}

	if generation.IsBYOK && generation.UpstreamInferenceCost != nil {
		statistics.Cost += *generation.UpstreamInferenceCost
	}

	nativeIn := Nullable(generation.NativeTokensPrompt, 0)
	normalIn := Nullable(generation.TokensPrompt, 0)

	statistics.InputTokens = max(nativeIn, normalIn)

	nativeOut := Nullable(generation.NativeTokensCompletion, 0) + Nullable(generation.NativeTokensReasoning, 0)
	normalOut := Nullable(generation.TokensCompletion, 0)

	statistics.OutputTokens = max(nativeOut, normalOut)

	RespondJson(w, http.StatusOK, statistics)
}

func Nullable[T any](ptr *T, def T) T {
	if ptr == nil {
		return def
	}

	return *ptr
}
