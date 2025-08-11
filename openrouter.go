package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/revrost/go-openrouter"
)

type Generation struct {
	ID                     string   `json:"id"`
	TotalCost              float64  `json:"total_cost"`
	CreatedAt              string   `json:"created_at"`
	Model                  string   `json:"model"`
	Origin                 string   `json:"origin"`
	Usage                  float64  `json:"usage"`
	IsBYOK                 bool     `json:"is_byok"`
	UpstreamID             *string  `json:"upstream_id"`
	CacheDiscount          *float64 `json:"cache_discount"`
	UpstreamInferenceCost  *float64 `json:"upstream_inference_cost"`
	AppID                  *int     `json:"app_id"`
	Streamed               *bool    `json:"streamed"`
	Cancelled              *bool    `json:"cancelled"`
	ProviderName           *string  `json:"provider_name"`
	Latency                *int     `json:"latency"`
	ModerationLatency      *int     `json:"moderation_latency"`
	GenerationTime         *int     `json:"generation_time"`
	FinishReason           *string  `json:"finish_reason"`
	NativeFinishReason     *string  `json:"native_finish_reason"`
	TokensPrompt           *int     `json:"tokens_prompt"`
	TokensCompletion       *int     `json:"tokens_completion"`
	NativeTokensPrompt     *int     `json:"native_tokens_prompt"`
	NativeTokensCompletion *int     `json:"native_tokens_completion"`
	NativeTokensReasoning  *int     `json:"native_tokens_reasoning"`
	NumMediaPrompt         *int     `json:"num_media_prompt"`
	NumMediaCompletion     *int     `json:"num_media_completion"`
	NumSearchResults       *int     `json:"num_search_results"`
}

func OpenRouterClient() *openrouter.Client {
	return openrouter.NewClient(OpenRouterToken)
}

func OpenRouterStartStream(ctx context.Context, request openrouter.ChatCompletionRequest) (*openrouter.ChatCompletionStream, error) {
	client := OpenRouterClient()

	stream, err := client.CreateChatCompletionStream(ctx, request)
	if err != nil {
		return nil, err
	}

	return stream, nil
}

func OpenRouterGetGeneration(ctx context.Context, id string) (*Generation, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://openrouter.ai/api/v1/generation?id=%s", id), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", OpenRouterToken))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}

	var response struct {
		Data Generation `json:"data"`
	}

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	return &response.Data, nil
}
