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
	Queries        []string `json:"queries"`
	Topic          string   `json:"topic,omitempty"`
	Depth          string   `json:"depth,omitempty"`
	TimeRange      string   `json:"time_range,omitempty"`
	StartDate      string   `json:"start_date,omitempty"`
	EndDate        string   `json:"end_date,omitempty"`
	MaxResults     int      `json:"max_results,omitempty"`
	IncludeDomains []string `json:"include_domains,omitempty"`
	ExcludeDomains []string `json:"exclude_domains,omitempty"`
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
				Description: "Search the live web via Tavily to discover relevant pages. Returns titles, URLs, and relevant content snippets ranked by relevance. Use this to find sources; use fetch_contents to read the full text of a specific URL.",
				Parameters: map[string]any{
					"type":     "object",
					"required": []string{"queries"},
					"properties": map[string]any{
						"queries": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
							"minItems":    1,
							"maxItems":    5,
							"description": "One or more concise, keyword-focused search queries (ideally 3-8 words each, like a search engine query rather than a sentence). For complex questions, decompose into several focused sub-queries that run in parallel; their results are merged and de-duplicated. Examples: ['tavily api pricing 2025'], ['rust async runtime comparison', 'tokio vs async-std performance'].",
						},
						"topic": map[string]any{
							"type":        "string",
							"enum":        []string{"general", "news", "finance"},
							"description": "Search topic. 'news' for recent events and current affairs, 'finance' for markets and financial data, 'general' for everything else. Default 'general'.",
						},
						"depth": map[string]any{
							"type":        "string",
							"enum":        []string{"quick", "thorough"},
							"description": "'quick' for fast, broad lookups (low latency); 'thorough' for harder questions needing the highest-quality, most relevant snippets. Default 'quick'.",
						},
						"time_range": map[string]any{
							"type":        "string",
							"enum":        []string{"day", "week", "month", "year"},
							"description": "Restrict results to content published within this recent window. Prefer this over start_date/end_date for relative recency.",
						},
						"start_date": map[string]any{
							"type":        "string",
							"description": "Filter results published ON OR AFTER this date (YYYY-MM-DD). Use for absolute ranges.",
						},
						"end_date": map[string]any{
							"type":        "string",
							"description": "Filter results published ON OR BEFORE this date (YYYY-MM-DD). Use for absolute ranges.",
						},
						"max_results": map[string]any{
							"type":        "integer",
							"description": "Number of results to return (1-20). Default is 5.",
							"minimum":     1,
							"maximum":     20,
						},
						"include_domains": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
							"description": "Restrict search to these specific website domains (e.g., ['europa.eu', 'who.int']).",
						},
						"exclude_domains": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
							"description": "Exclude results from these website domains.",
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
				Description: "Fetch the full text content of one or more specific URLs via Tavily. Use this to read pages in depth, including links found via search_web or provided by the user.",
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
	if len(arguments.Queries) == 0 {
		return errors.New("no search query")
	}

	results, err := TavilyRunSearch(ctx, arguments)
	if err != nil {
		tool.Result = fmt.Sprintf("error: %v", err)

		return nil
	}

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

	results, err := TavilyRunContents(ctx, arguments)
	if err != nil {
		tool.Result = fmt.Sprintf("error: %v", err)

		return nil
	}

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

	// Some models are a bit confused by numbers so we unwrap "6" -> 6.
	// Only unwrap object values (preceded by a colon) so we don't corrupt
	// string array elements like queries: ["2024", ...].
	rgx := regexp.MustCompile(`(:\s*)"(\d+)"`)
	tool.Args = rgx.ReplaceAllString(tool.Args, `${1}${2}`)

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
