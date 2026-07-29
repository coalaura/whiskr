package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/coalaura/openingrouter"
)

func OpenRouterClient(proxy *EnvProxy) *openingrouter.Client {
	transport := http.DefaultTransport
	if proxy != nil {
		transport = proxy.transport
	}

	return openingrouter.NewClient(
		env.Tokens.OpenRouter,
		openingrouter.WithTitle("Whiskr"),
		openingrouter.WithReferer("https://github.com/coalaura/whiskr"),
		openingrouter.WithClient(&http.Client{
			Timeout:   time.Duration(env.Settings.Timeout) * time.Second,
			Transport: transport,
		}),
	)
}

func OpenRouterStartStream(ctx context.Context, request openingrouter.ChatCompletionRequest, proxy *EnvProxy) (openingrouter.OpenrouterStream[openingrouter.ChatStreamChunk], error) {
	client := OpenRouterClient(proxy)

	stream, err := client.CreateChatCompletionStream(ctx, request)
	if err != nil {
		log.Warnln(err)

		return nil, err
	}

	return stream, nil
}

func OpenRouterRun(ctx context.Context, request openingrouter.ChatCompletionRequest, proxy *EnvProxy) (openingrouter.ChatCompletionResponse, error) {
	client := OpenRouterClient(proxy)

	response, err := client.CreateChatCompletion(ctx, request)
	if err != nil {
		log.Warnln(err)

		return openingrouter.ChatCompletionResponse{}, err
	}

	if len(response.Choices) == 0 {
		return *response, errors.New("no choices")
	}

	return *response, nil
}

// Generation is the openrouter.ai generation lookup payload.
// openingrouter does not expose GetGeneration; we call the endpoint via Client.NewRequest.
type Generation struct {
	ID                     string   `json:"id"`
	TotalCost              float64  `json:"total_cost"`
	CreatedAt              string   `json:"created_at"`
	Model                  string   `json:"model"`
	Origin                 string   `json:"origin"`
	Usage                  float64  `json:"usage"`
	IsBYOK                 bool     `json:"is_byok"`
	UpstreamID             *string  `json:"upstream_id,omitempty"`
	CacheDiscount          *float64 `json:"cache_discount,omitempty"`
	UpstreamInferenceCost  *float64 `json:"upstream_inference_cost,omitempty"`
	AppID                  *int     `json:"app_id,omitempty"`
	Streamed               *bool    `json:"streamed,omitempty"`
	Cancelled              *bool    `json:"cancelled,omitempty"`
	ProviderName           *string  `json:"provider_name,omitempty"`
	Latency                *int     `json:"latency,omitempty"`
	ModerationLatency      *int     `json:"moderation_latency,omitempty"`
	GenerationTime         *int     `json:"generation_time,omitempty"`
	FinishReason           *string  `json:"finish_reason,omitempty"`
	NativeFinishReason     *string  `json:"native_finish_reason,omitempty"`
	TokensPrompt           *int     `json:"tokens_prompt,omitempty"`
	TokensCompletion       *int     `json:"tokens_completion,omitempty"`
	NativeTokensPrompt     *int     `json:"native_tokens_prompt,omitempty"`
	NativeTokensCompletion *int     `json:"native_tokens_completion,omitempty"`
	NativeTokensReasoning  *int     `json:"native_tokens_reasoning,omitempty"`
	NumMediaPrompt         *int     `json:"num_media_prompt,omitempty"`
	NumMediaCompletion     *int     `json:"num_media_completion,omitempty"`
	NumSearchResults       *int     `json:"num_search_results,omitempty"`
}

func OpenRouterGetGeneration(ctx context.Context, id string) (Generation, error) {
	client := OpenRouterClient(nil)

	req, err := client.NewRequest(ctx, http.MethodGet, "generation", struct {
		ID string `url:"id"`
	}{ID: id})
	if err != nil {
		return Generation{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return Generation{}, err
	}
	defer resp.Body.Close()

	var result openingrouter.OpenRouterResponse[Generation]

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Generation{}, err
	}

	return result.Data, nil
}

func OpenRouterListModels(ctx context.Context) (map[string]openingrouter.Model, error) {
	client := OpenRouterClient(nil)

	models, err := client.ListModels(ctx, nil)
	if err != nil {
		return nil, err
	}

	mp := make(map[string]openingrouter.Model, len(models))

	for _, model := range models {
		mp[model.ID] = model
	}

	return mp, nil
}

func streamProvider(meta *openingrouter.OpenRouterMetadata) string {
	if meta == nil {
		return ""
	}

	for _, ep := range meta.Endpoints.Available {
		if ep.Selected {
			return ep.Provider
		}
	}

	return ""
}
