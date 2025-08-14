package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/revrost/go-openrouter"
)

type SearchArguments struct {
	Query string `json:"query"`
}

var (
	//go:embed prompts/search.txt
	PromptSearch string
)

func GetSearchTool() []openrouter.Tool {
	return []openrouter.Tool{
		{
			Type: openrouter.ToolTypeFunction,
			Function: &openrouter.FunctionDefinition{
				Name:        "search_internet",
				Description: "Search the internet for current information.",
				Parameters: map[string]any{
					"type":     "object",
					"required": []string{"query"},
					"properties": map[string]any{
						"query": map[string]string{
							"type":        "string",
							"description": "A concise and specific query string.",
						},
					},
					"additionalProperties": false,
				},
				Strict: true,
			},
		},
	}
}

func HandleSearchTool(ctx context.Context, tool *ToolCall) error {
	var arguments SearchArguments

	err := json.Unmarshal([]byte(tool.Args), &arguments)
	if err != nil {
		return err
	}

	if arguments.Query == "" {
		return errors.New("no search query")
	}

	request := openrouter.ChatCompletionRequest{
		Model: "perplexity/sonar",
		Messages: []openrouter.ChatCompletionMessage{
			openrouter.SystemMessage(PromptSearch),
			openrouter.UserMessage(arguments.Query),
		},
		Temperature: 0.25,
		MaxTokens:   2048,
	}

	response, err := OpenRouterRun(ctx, request)
	if err != nil {
		tool.Result = fmt.Sprintf("error: %v", err)

		return nil
	}

	if len(response.Choices) == 0 {
		tool.Result = "error: failed to perform search"

		return nil
	}

	tool.Result = response.Choices[0].Message.Content.Text

	return nil
}
