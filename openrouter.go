package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/coalaura/openingrouter"
)

// NewHttpClient builds the shared http client honoring the proxy and timeout config.
func NewHttpClient(proxy *EnvProxy) *http.Client {
	transport := http.DefaultTransport

	if proxy != nil {
		transport = proxy.transport
	}

	return &http.Client{
		Timeout:   time.Duration(env.Settings.Timeout) * time.Second,
		Transport: transport,
	}
}

// OpenRouterClient returns the openrouter client used for openrouter-only endpoints.
func OpenRouterClient(proxy *EnvProxy) *openingrouter.Client {
	options := []openingrouter.Option{
		openingrouter.WithTitle("Whiskr"),
		openingrouter.WithReferer("https://github.com/coalaura/whiskr"),
		openingrouter.WithClient(NewHttpClient(proxy)),
		openingrouter.WithBase(env.LLM.BaseURL),
	}

	return openingrouter.NewClient(env.Tokens.OpenRouter, options...)
}

// OpenAIClient returns the openai-compatible client for the configured token and base url.
func OpenAIClient(proxy *EnvProxy) *openingrouter.OpenAIClient {
	options := []openingrouter.OpenAIOption{
		openingrouter.WithOpenAIHTTPClient(NewHttpClient(proxy)),
		openingrouter.WithOpenAIBase(env.LLM.BaseURL),
	}

	return openingrouter.NewOpenAIClient(env.Tokens.OpenAI, options...)
}

// NewCompatibleClient returns the chat client for the configured llm api, targeting either
// openrouter or an openai-compatible endpoint.
func NewCompatibleClient(proxy *EnvProxy) openingrouter.OpenAICompatibleClient {
	if env.IsOpenAI() {
		return OpenAIClient(proxy)
	}

	return OpenRouterClient(proxy)
}

func OpenRouterStartStream(ctx context.Context, request openingrouter.ChatCompletionRequest, proxy *EnvProxy) (openingrouter.OpenrouterStream[openingrouter.ChatStreamChunk], error) {
	client := NewCompatibleClient(proxy)

	stream, err := client.CreateChatCompletionStream(ctx, request)
	if err != nil {
		log.Warnln(err)

		return nil, err
	}

	return stream, nil
}

func OpenRouterRun(ctx context.Context, request openingrouter.ChatCompletionRequest, proxy *EnvProxy) (openingrouter.ChatCompletionResponse, error) {
	client := NewCompatibleClient(proxy)

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

func OpenRouterListModels(ctx context.Context) (map[string]openingrouter.Model, error) {
	client := NewCompatibleClient(nil)

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
