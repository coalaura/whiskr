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

func OpenRouterClient(proxy *EnvProxy) *openrouter.Client {
	cc := openrouter.DefaultConfig(env.Tokens.OpenRouter)

	cc.XTitle = "Whiskr"
	cc.HttpReferer = "https://github.com/coalaura/whiskr"

	transport := http.DefaultTransport
	if proxy != nil {
		transport = &ProxyTransport{
			Inner: http.DefaultTransport,
			Host:  proxy.Host,
			Token: proxy.Token,
		}
	}

	cc.HTTPClient = &http.Client{
		Timeout:   time.Duration(env.Settings.Timeout) * time.Second,
		Transport: transport,
	}

	return openrouter.NewClientWithConfig(*cc)
}

func OpenRouterStartStream(ctx context.Context, request openrouter.ChatCompletionRequest, proxy *EnvProxy) (*openrouter.ChatCompletionStream, error) {
	client := OpenRouterClient(proxy)

	stream, err := client.CreateChatCompletionStream(ctx, request)
	if err != nil {
		log.Warnln(err)

		return nil, err
	}

	return stream, nil
}

func OpenRouterRun(ctx context.Context, request openrouter.ChatCompletionRequest, proxy *EnvProxy) (openrouter.ChatCompletionResponse, error) {
	client := OpenRouterClient(proxy)

	response, err := client.CreateChatCompletion(ctx, request)
	if err != nil {
		log.Warnln(err)

		return response, err
	}

	if len(response.Choices) == 0 {
		return response, errors.New("no choices")
	}

	return response, nil
}

func OpenRouterGetGeneration(ctx context.Context, id string) (openrouter.Generation, error) {
	client := OpenRouterClient(nil)

	return client.GetGeneration(ctx, id)
}

func OpenRouterListModels(ctx context.Context) (map[string]openrouter.Model, error) {
	client := OpenRouterClient(nil)

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
