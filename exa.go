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
	buf := GetFreeBuffer()
	defer pool.Put(buf)

	json.NewEncoder(buf).Encode(map[string]any{
		"results": e.Results,
	})

	return buf.String()
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

func ExaRunSearch(ctx context.Context, args *SearchWebArguments) (*ExaResults, error) {
	if args.NumResults <= 0 {
		args.NumResults = 6
	} else if args.NumResults < 3 {
		args.NumResults = 3
	} else if args.NumResults >= 12 {
		args.NumResults = 12
	}

	guidance := ExaGuidanceForIntent(args)

	data := map[string]any{
		"query":      args.Query,
		"type":       "auto",
		"numResults": args.NumResults,
	}

	if len(args.Domains) > 0 {
		data["includeDomains"] = args.Domains
	}

	contents := map[string]any{
		"summary": map[string]any{
			"query": guidance,
		},
		"livecrawl": "preferred",
	}

	highlights := map[string]any{
		"numSentences":     2,
		"highlightsPerUrl": 3,
		"query":            guidance,
	}

	switch args.Intent {
	case "news":
		highlights["highlightsPerUrl"] = 2

		data["category"] = "news"
		data["numResults"] = max(8, args.NumResults)
		data["startPublishedDate"] = daysAgo(30)
	case "docs":
		highlights["numSentences"] = 3
		highlights["highlightsPerUrl"] = 4

		contents["subpages"] = 1
		contents["subpageTarget"] = []string{"documentation", "changelog", "release notes"}
	case "papers":
		highlights["numSentences"] = 4
		highlights["highlightsPerUrl"] = 4

		data["category"] = "research paper"
		data["startPublishedDate"] = daysAgo(365 * 2)
	case "code":
		highlights["highlightsPerUrl"] = 4

		contents["subpages"] = 1
		contents["subpageTarget"] = []string{"readme", "changelog", "code"}
		contents["text"] = map[string]any{
			"maxCharacters": 8000,
		}

		data["category"] = "github"
	case "deep_read":
		highlights["numSentences"] = 3
		highlights["highlightsPerUrl"] = 5

		contents["text"] = map[string]any{
			"maxCharacters": 12000,
		}
	}

	contents["highlights"] = highlights

	data["contents"] = contents

	switch args.Recency {
	case "month":
		data["startPublishedDate"] = daysAgo(30)
	case "year":
		data["startPublishedDate"] = daysAgo(365)
	}

	req, err := NewExaRequest(ctx, "/search", data)
	if err != nil {
		return nil, err
	}

	return RunExaRequest(req)
}

func ExaRunContents(ctx context.Context, args *FetchContentsArguments) (*ExaResults, error) {
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
	return time.Now().Add(-time.Duration(days) * 24 * time.Hour).Format(time.DateOnly)
}

func ExaGuidanceForIntent(args *SearchWebArguments) string {
	var recency string

	switch args.Recency {
	case "month":
		recency = " since " + daysAgo(30)
	case "year":
		recency = " since " + daysAgo(365)
	}

	goal := strings.TrimSpace(args.Query)

	switch args.Intent {
	case "news":
		return "Give who/what/when/where and key numbers" + recency +
			". Include dates and named sources; 2-4 bullets. Note disagreements. Ignore speculation."
	case "docs":
		return "Extract install command, minimal example, breaking changes" + recency + ", key config options with defaults, and deprecations. Prefer official docs and release notes."
	case "papers":
		return "Summarize problem, method, dataset, metrics (with numbers), baselines, novelty, and limitations; include year/venue."
	case "code":
		return "Summarize repo purpose, language, license, last release/commit" + recency + ", install steps and minimal example; note breaking changes. Prefer README/docs."
	case "deep_read":
		return "Answer: " + goal + ". Extract exact numbers, dates, quotes (with speaker) plus 1-2 sentences of context."
	}

	return "Focus on answering: " + goal + ". Provide dates, versions, key numbers; 3-5 concise bullets. Ignore marketing fluff."
}
