package main

import (
	"github.com/revrost/go-openrouter"
)

type Statistics struct {
	Provider     string  `msgpack:"provider"`
	Model        string  `msgpack:"model"`
	Cost         float64 `msgpack:"cost"`
	InputTokens  int     `msgpack:"input"`
	OutputTokens int     `msgpack:"output"`
}

func CreateStatistics(model, provider string, usage *openrouter.Usage) *Statistics {
	statistics := Statistics{
		Provider:     provider,
		Model:        model,
		Cost:         usage.Cost,
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
	}

	if usage.IsBYOK {
		statistics.Cost += usage.CostDetails.UpstreamInferenceCost
	}

	return &statistics
}

func Nullable[T any](ptr *T, def T) T {
	if ptr == nil {
		return def
	}

	return *ptr
}
