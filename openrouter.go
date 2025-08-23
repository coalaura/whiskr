package main

import (
	"context"

	"github.com/revrost/go-openrouter"
)

func init() {
	openrouter.DisableLogs()
}

func OpenRouterClient() *openrouter.Client {
	return openrouter.NewClient(env.Tokens.OpenRouter, openrouter.WithXTitle("Whiskr"), openrouter.WithHTTPReferer("https://github.com/coalaura/whiskr"))
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

	return client.CreateChatCompletion(ctx, request)
}

func OpenRouterGetGeneration(ctx context.Context, id string) (openrouter.Generation, error) {
	client := OpenRouterClient()

	return client.GetGeneration(ctx, id)
}
