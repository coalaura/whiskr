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
	Query      string   `json:"query"`
	NumResults int      `json:"num_results,omitempty"`
	Intent     string   `json:"intent,omitempty"`
	Recency    string   `json:"recency,omitempty"`
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
				Description: "Search the live web (via Exa /search) and return summaries, highlights, and optionally full text for the top results.",
				Parameters: map[string]any{
					"type":     "object",
					"required": []string{"query"},
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "A concise, specific search query in natural language. Include month/year if recency matters (e.g., 'august 2025').",
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
							"description": "Search profile. Use 'news' for breaking topics, 'docs' for official docs/changelogs, 'papers' for research, 'code' for repos, 'deep_read' when you need exact quotes/numbers (adds full text). Default 'auto'.",
						},
						"recency": map[string]any{
							"type":        "string",
							"enum":        []string{"auto", "month", "year", "range"},
							"description": "Time filter hint. 'month' ~ last 30 days, 'year' ~ last 365 days. Default 'auto'.",
						},
						"domains": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
							"description": "Restrict to these domains (e.g., ['europa.eu', 'who.int']).",
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

func HandleSearchWebTool(ctx context.Context, tool *ToolCall) error {
	var arguments SearchWebArguments

	err := ParseAndUpdateArgs(tool, &arguments)
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

	err := ParseAndUpdateArgs(tool, &arguments)
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

	err := ParseAndUpdateArgs(tool, &arguments)
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

func ParseAndUpdateArgs(tool *ToolCall, arguments any) error {
	err := json.Unmarshal([]byte(tool.Args), arguments)
	if err != nil {
		return fmt.Errorf("json.unmarshal: %v", err)
	}

	buf := GetFreeBuffer()
	defer pool.Put(buf)

	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)

	err = enc.Encode(arguments)
	if err != nil {
		return fmt.Errorf("json.marshal: %v", err)
	}

	tool.Args = buf.String()

	return nil
}
