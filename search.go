package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/revrost/go-openrouter"
)

type SearchWebArguments struct {
	Query      string `json:"query"`
	NumResults int    `json:"num_results"`
}

type FetchContentsArguments struct {
	URLs []string `json:"urls"`
}

type GitHubRepositoryArguments struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

func GetSearchTools() []openrouter.Tool {
	return []openrouter.Tool{
		{
			Type: openrouter.ToolTypeFunction,
			Function: &openrouter.FunctionDefinition{
				Name:        "search_web",
				Description: "Search the web via Exa in auto mode. Returns up to 10 results with short summaries.",
				Parameters: map[string]any{
					"type":     "object",
					"required": []string{"query", "num_results"},
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "A concise, specific search query in natural language.",
						},
						"num_results": map[string]any{
							"type":        "integer",
							"description": "Number of results to return (3-10). Default to 6.",
							"minimum":     3,
							"maximum":     10,
						},
					},
					"additionalProperties": false,
				},
				Strict: true,
			},
		},
		{
			Type: openrouter.ToolTypeFunction,
			Function: &openrouter.FunctionDefinition{
				Name:        "fetch_contents",
				Description: "Fetch page contents for one or more URLs via Exa /contents.",
				Parameters: map[string]any{
					"type":     "object",
					"required": []string{"urls"},
					"properties": map[string]any{
						"urls": map[string]any{
							"type":        "array",
							"description": "List of URLs (1..N) to fetch.",
							"items":       map[string]any{"type": "string"},
						},
					},
					"additionalProperties": false,
				},
				Strict: true,
			},
		},
		{
			Type: openrouter.ToolTypeFunction,
			Function: &openrouter.FunctionDefinition{
				Name:        "github_repository",
				Description: "Get a quick overview of a GitHub repository without cloning: repo info, up to 20 branches (popular first), top-level files/dirs, and the README.",
				Parameters: map[string]any{
					"type":     "object",
					"required": []string{"owner", "repo"},
					"properties": map[string]any{
						"owner": map[string]any{
							"type":        "string",
							"description": "GitHub username or organization (e.g., 'torvalds').",
						},
						"repo": map[string]any{
							"type":        "string",
							"description": "Repository name (e.g., 'linux').",
						},
					},
					"additionalProperties": false,
				},
				Strict: true,
			},
		},
	}
}

func HandleSearchWebTool(ctx context.Context, tool *ToolCall) error {
	var arguments SearchWebArguments

	err := json.Unmarshal([]byte(tool.Args), &arguments)
	if err != nil {
		return err
	}

	if arguments.Query == "" {
		return errors.New("no search query")
	}

	results, err := ExaRunSearch(ctx, arguments)
	if err != nil {
		tool.Result = fmt.Sprintf("error: %v", err)

		return nil
	}

	tool.Cost = results.Cost.Total

	if len(results.Results) == 0 {
		tool.Result = "error: no search results"

		return nil
	}

	tool.Result = results.String()

	return nil
}

func HandleFetchContentsTool(ctx context.Context, tool *ToolCall) error {
	var arguments FetchContentsArguments

	err := json.Unmarshal([]byte(tool.Args), &arguments)
	if err != nil {
		return err
	}

	if len(arguments.URLs) == 0 {
		return errors.New("no urls")
	}

	results, err := ExaRunContents(ctx, arguments)
	if err != nil {
		tool.Result = fmt.Sprintf("error: %v", err)

		return nil
	}

	tool.Cost = results.Cost.Total

	if len(results.Results) == 0 {
		tool.Result = "error: no search results"

		return nil
	}

	tool.Result = results.String()

	return nil
}

func HandleGitHubRepositoryTool(ctx context.Context, tool *ToolCall) error {
	var arguments GitHubRepositoryArguments

	err := json.Unmarshal([]byte(tool.Args), &arguments)
	if err != nil {
		return err
	}

	result, err := RepoOverview(ctx, arguments)
	if err != nil {
		tool.Result = fmt.Sprintf("error: %v", err)

		return nil
	}

	tool.Result = result

	return nil
}
