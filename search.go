package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/revrost/go-openrouter"
)

type SearchWebArguments struct {
	Query      string   `json:"query"`
	NumResults int      `json:"num_results,omitempty"`
	Intent     string   `json:"intent,omitempty"`
	StartDate  string   `json:"start_date,omitempty"`
	EndDate    string   `json:"end_date,omitempty"`
	Domains    []string `json:"domains,omitempty"`
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
				Description: "Search the live web via Exa. Returns highly relevant highlights and text snippets.",
				Parameters: map[string]any{
					"type":     "object",
					"required": []string{"query"},
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "A concise, specific search query. Focus on core entities and keywords.",
						},
						"num_results": map[string]any{
							"type":        "integer",
							"description": "Number of results to return (3-12). Default is 6.",
							"minimum":     3,
							"maximum":     12,
						},
						"intent": map[string]any{
							"type":        "string",
							"enum":        []string{"auto", "news", "docs", "papers", "code", "deep_read"},
							"description": "Category filter. 'news' (recent events), 'docs' (official documentation), 'papers' (academic), 'code' (GitHub), 'deep_read' (fetches full page text instead of just highlights). Default 'auto'.",
						},
						"start_date": map[string]any{
							"type":        "string",
							"description": "Filter results published AFTER this date (YYYY-MM-DD).",
						},
						"end_date": map[string]any{
							"type":        "string",
							"description": "Filter results published BEFORE this date (YYYY-MM-DD).",
						},
						"domains": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
							"description": "Restrict search to these specific website domains (e.g., ['europa.eu', 'who.int']).",
						},
					},
					"additionalProperties": false,
				},
			},
		},
		{
			Type: openrouter.ToolTypeFunction,
			Function: &openrouter.FunctionDefinition{
				Name:        "fetch_contents",
				Description: "Fetch and summarize page contents for one or more URLs (via Exa /contents). Use when the user provides specific links.",
				Parameters: map[string]any{
					"type":     "object",
					"required": []string{"urls"},
					"properties": map[string]any{
						"urls": map[string]any{
							"type":        "array",
							"description": "List of URLs to fetch.",
							"items": map[string]any{
								"type": "string",
							},
							"minItems": 1,
							"maxItems": 5,
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
				Description: "Fetch repository metadata and README from GitHub.",
				Parameters: map[string]any{
					"type":     "object",
					"required": []string{"owner", "repo"},
					"properties": map[string]any{
						"owner": map[string]any{
							"type":        "string",
							"description": "Repository owner (e.g., 'torvalds').",
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

func HandleSearchWebTool(ctx context.Context, tool *ChatToolCall, arguments *SearchWebArguments) error {
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

func HandleFetchContentsTool(ctx context.Context, tool *ChatToolCall, arguments *FetchContentsArguments) error {
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

func HandleGitHubRepositoryTool(ctx context.Context, tool *ChatToolCall, arguments *GitHubRepositoryArguments) error {
	result, err := RepoOverview(ctx, arguments)
	if err != nil {
		tool.Result = fmt.Sprintf("error: %v", err)

		return nil
	}

	tool.Result = result

	return nil
}

func ParseAndUpdateArgs[T any](tool *ChatToolCall) (*T, error) {
	var arguments T

	// Some models are a bit confused by numbers so we unwrap "6" -> 6
	rgx := regexp.MustCompile(`"(\d+)"`)
	tool.Args = rgx.ReplaceAllString(tool.Args, "$1")

	err := json.Unmarshal([]byte(tool.Args), &arguments)
	if err != nil {
		return nil, fmt.Errorf("json.unmarshal: %v", err)
	}

	buf := GetFreeBuffer()
	defer pool.Put(buf)

	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)

	err = enc.Encode(&arguments)
	if err != nil {
		return nil, fmt.Errorf("json.marshal: %v", err)
	}

	tool.Args = buf.String()

	return &arguments, nil
}
