package main

import (
	"github.com/coalaura/openingrouter"
)

type Statistics struct {
	Provider         string  `msgpack:"provider"`
	Model            string  `msgpack:"model"`
	Cost             float64 `msgpack:"cost"`
	InputTokens      int     `msgpack:"input"`
	OutputTokens     int     `msgpack:"output"`
	ReasoningTokens  int     `msgpack:"reasoning"`
	CachedTokens     int     `msgpack:"cached"`
	CacheWriteTokens int     `msgpack:"cache_write"`
}

func CreateStatistics(model, provider string, usage *openingrouter.ChatUsage) *Statistics {
	statistics := Statistics{
		Provider:     provider,
		Model:        model,
		Cost:         Nullable(usage.Cost, 0),
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
	}

	if usage.CompletionTokensDetails != nil {
		statistics.ReasoningTokens = Nullable(usage.CompletionTokensDetails.ReasoningTokens, 0)
	}

	if usage.PromptTokensDetails != nil {
		statistics.CachedTokens = Nullable(usage.PromptTokensDetails.CachedTokens, 0)
		statistics.CacheWriteTokens = Nullable(usage.PromptTokensDetails.CacheWriteTokens, 0)
	}

	if usage.IsBYOK && usage.CostDetails != nil {
		statistics.Cost += Nullable(usage.CostDetails.UpstreamInferenceCost, 0)
	}

	return &statistics
}

func Nullable[T any](ptr *T, def T) T {
	if ptr == nil {
		return def
	}

	return *ptr
}
