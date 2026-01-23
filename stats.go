package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jellydator/ttlcache/v3"
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

type StatisticsEntry struct {
	Found *Statistics
	Error error
}

var statisticsCache = ttlcache.New(
	ttlcache.WithTTL[string, StatisticsEntry](30 * time.Minute),
)

func HandleStats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if id == "" || !strings.HasPrefix(id, "gen-") {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": "invalid id",
		})

		return
	}

	entry, _ := statisticsCache.GetOrSetFunc(id, func() StatisticsEntry {
		var entry StatisticsEntry

		statistics, err := FetchStatistics(r.Context(), id)
		if err != nil {
			if !strings.HasSuffix(err.Error(), "not found") {
				entry.Error = err
			}
		} else {
			entry.Found = statistics
		}

		return entry
	})

	value := entry.Value()

	if value.Found != nil {
		RespondJson(w, http.StatusOK, value.Found)
	} else if value.Error != nil {
		RespondJson(w, http.StatusInternalServerError, map[string]any{
			"error": value.Error,
		})
	} else {
		RespondJson(w, http.StatusNotFound, map[string]any{
			"error": "not found",
		})
	}
}

func FetchStatistics(ctx context.Context, id string) (*Statistics, error) {
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

		attempt++

		time.Sleep(backoff)

		backoff = min(4*time.Second, backoff*2)
	}

	if err != nil {
		return nil, err
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

	return &statistics, nil
}

func Nullable[T any](ptr *T, def T) T {
	if ptr == nil {
		return def
	}

	return *ptr
}
