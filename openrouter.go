package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/revrost/go-openrouter"
)

func init() {
	openrouter.DisableLogs()
}

func OpenRouterClient() *openrouter.Client {
	cc := openrouter.DefaultConfig(env.Tokens.OpenRouter)

	cc.XTitle = "Whiskr"
	cc.HttpReferer = "https://github.com/coalaura/whiskr"

	cc.HTTPClient = &http.Client{
		Timeout: time.Duration(env.Settings.Timeout) * time.Second,
	}

	return openrouter.NewClientWithConfig(*cc)
}

func OpenRouterStartStream(ctx context.Context, request openrouter.ChatCompletionRequest) (*openrouter.ChatCompletionStream, error) {
	client := OpenRouterClient()

	stream, err := client.CreateChatCompletionStream(ctx, request)
	if err != nil {
		return nil, err
	}

	return stream, nil
}

func OpenRouterRun(ctx context.Context, request openrouter.ChatCompletionRequest) (openrouter.ChatCompletionResponse, error) {
	client := OpenRouterClient()

	response, err := client.CreateChatCompletion(ctx, request)
	if err != nil {
		return response, err
	}

	if len(response.Choices) == 0 {
		return response, errors.New("no choices")
	}

	return response, nil
}

func OpenRouterGetGeneration(ctx context.Context, id string) (openrouter.Generation, error) {
	client := OpenRouterClient()

	return client.GetGeneration(ctx, id)
}

func OpenRouterListModels(ctx context.Context) (map[string]openrouter.Model, error) {
	client := OpenRouterClient()

	models, err := client.ListModels(ctx)
	if err != nil {
		return nil, err
	}

	mp := make(map[string]openrouter.Model, len(models))

	for _, model := range models {
		mp[model.ID] = model
	}

	return mp, nil
}
