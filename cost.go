package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/coalaura/openingrouter"
)

func EstimateContextCost(ctx context.Context, request *openingrouter.ChatCompletionRequest, provider string, tokenizer *Tokenizer) (float64, error) {
	model := GetModel(request.Model)
	if model == nil {
		return 0, fmt.Errorf("unknown model: %q", request.Model)
	}

	price := model.Pricing.Input

	if provider != "" && !env.IsOpenAI() {
		providers, err := GetModelProviders(ctx, model.Slug)
		if err != nil {
			return 0, fmt.Errorf("estimate context cost: %w", err)
		}

		var found bool

		for index := range providers {
			if providers[index].Slug == provider {
				price = providers[index].Input
				found = true

				break
			}
		}

		if !found {
			return 0, fmt.Errorf("cannot estimate context cost for provider %q", provider)
		}
	}

	if price <= 0 {
		return 0, nil
	}

	tokens, err := countContextTokens(request, tokenizer)
	if err != nil {
		return 0, err
	}

	return float64(tokens) * price / 1000000, nil
}

func countContextTokens(request *openingrouter.ChatCompletionRequest, tokenizer *Tokenizer) (int, error) {
	tokens := 5

	for index := range request.Messages {
		message := &request.Messages[index]

		tokens += 5 + tokenizer.CountTokens(message.Content.Text)
		tokens += tokenizer.CountTokens(message.Name)
		tokens += tokenizer.CountTokens(message.ToolCallID)

		for partIndex := range message.Content.Parts {
			part := &message.Content.Parts[partIndex]
			tokens += tokenizer.CountTokens(part.Text)

			if part.ImageURL != nil {
				// upper image-token estimate without tokenizing base64 data.
				tokens += 2500
			}
		}

		tokens += len(message.Images) * 2500

		for callIndex := range message.ToolCalls {
			call := &message.ToolCalls[callIndex]

			tokens += 5 + tokenizer.CountTokens(call.ID)
			tokens += tokenizer.CountTokens(call.Function.Name)
			tokens += tokenizer.CountTokens(call.Function.Arguments)
		}
	}

	if len(request.Tools) > 0 {
		tools, err := json.Marshal(request.Tools)
		if err != nil {
			return 0, fmt.Errorf("estimate tool definition tokens: %w", err)
		}

		tokens += tokenizer.CountTokens(string(tools))
	}

	return tokens, nil
}
