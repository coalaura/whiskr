package main

import (
	"context"

	"github.com/revrost/go-openrouter"
)

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
