package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type ExaResult struct {
	Title         string   `json:"title"`
	URL           string   `json:"url"`
	PublishedDate string   `json:"publishedDate,omitempty"`
	SiteName      string   `json:"siteName,omitempty"`
	Summary       string   `json:"summary,omitempty"`
	Highlights    []string `json:"highlights,omitempty"`
	Text          string   `json:"text,omitempty"`
}

type ExaCost struct {
	Total float64 `json:"total"`
}

type ExaResults struct {
	RequestID  string      `json:"requestId"`
	SearchType string      `json:"resolvedSearchType"`
	Results    []ExaResult `json:"results"`
	Cost       ExaCost     `json:"costDollars"`
}

func (e *ExaResults) String() string {
	var builder strings.Builder

	json.NewEncoder(&builder).Encode(map[string]any{
		"results": e.Results,
	})

	return builder.String()
}

func NewExaRequest(ctx context.Context, path string, data any) (*http.Request, error) {
	buf, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("https://api.exa.ai%s", path), bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", env.Tokens.Exa)

	return req, nil
}

func RunExaRequest(req *http.Request) (*ExaResults, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var result ExaResults

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func ExaRunSearch(ctx context.Context, args SearchWebArguments) (*ExaResults, error) {
	if args.NumResults <= 0 {
		args.NumResults = 6
	} else if args.NumResults < 3 {
		args.NumResults = 3
	} else if args.NumResults >= 12 {
		args.NumResults = 12
	}

	data := map[string]any{
		"query":      args.Query,
		"type":       "auto",
		"numResults": args.NumResults,
	}

	if len(args.Domains) > 0 {
		data["includeDomains"] = args.Domains
	}

	contents := map[string]any{
		"summary": map[string]any{},
		"highlights": map[string]any{
			"numSentences":     2,
			"highlightsPerUrl": 3,
		},
		"livecrawl": "preferred",
	}

	switch args.Intent {
	case "news":
		data["category"] = "news"
		data["numResults"] = max(8, args.NumResults)
		data["startPublishedDate"] = daysAgo(30)
	case "docs":
		contents["subpages"] = 1
		contents["subpageTarget"] = []string{"documentation", "changelog", "release notes"}
	case "papers":
		data["category"] = "research paper"
		data["startPublishedDate"] = daysAgo(365 * 2)
	case "code":
		data["category"] = "github"

		contents["subpages"] = 1
		contents["subpageTarget"] = []string{"readme", "changelog", "code"}
	case "deep_read":
		contents["text"] = map[string]any{
			"maxCharacters": 8000,
		}
	}

	data["contents"] = contents

	switch args.Recency {
	case "month":
		data["startPublishedDate"] = daysAgo(30)
	case "year":
		data["startPublishedDate"] = daysAgo(356)
	}

	req, err := NewExaRequest(ctx, "/search", data)
	if err != nil {
		return nil, err
	}

	return RunExaRequest(req)
}

func ExaRunContents(ctx context.Context, args FetchContentsArguments) (*ExaResults, error) {
	data := map[string]any{
		"urls":    args.URLs,
		"summary": map[string]any{},
		"highlights": map[string]any{
			"numSentences":     2,
			"highlightsPerUrl": 3,
		},
		"text": map[string]any{
			"maxCharacters": 8000,
		},
		"livecrawl": "preferred",
	}

	req, err := NewExaRequest(ctx, "/contents", data)
	if err != nil {
		return nil, err
	}

	return RunExaRequest(req)
}

func daysAgo(days int) string {
	return time.Now().Add(time.Duration(days) * 24 * time.Hour).Format(time.DateOnly)
}
