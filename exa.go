package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type ExaResult struct {
	Title         string `json:"title"`
	URL           string `json:"url"`
	PublishedDate string `json:"publishedDate"`

	Text    string `json:"text"`
	Summary string `json:"summary"`
}

type ExaCost struct {
	Total float64 `json:"total"`
}

type ExaResults struct {
	RequestID string      `json:"requestId"`
	Results   []ExaResult `json:"results"`
	Cost      ExaCost     `json:"costDollars"`
}

func (e *ExaResult) String() string {
	var (
		label string
		text  string
	)

	if e.Text != "" {
		label = "Text"
		text = e.Text
	} else if e.Summary != "" {
		label = "Summary"
		text = e.Summary
	}

	return fmt.Sprintf(
		"Title: %s  \nURL: %s  \nPublished Date: %s  \n%s: %s",
		e.Title,
		e.URL,
		e.PublishedDate,
		label,
		strings.TrimSpace(text),
	)
}

func (e *ExaResults) String() string {
	list := make([]string, len(e.Results))

	for i, result := range e.Results {
		list[i] = result.String()
	}

	return strings.Join(list, "\n\n---\n\n")
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
	data := map[string]any{
		"query":      args.Query,
		"type":       "auto",
		"numResults": args.NumResults,
		"contents": map[string]any{
			"summary": map[string]any{
				"query": "Summarize this page only with all information directly relevant to answering the user's question: include key facts, numbers, dates, names, definitions, steps, code or commands, and the page's stance or conclusion; omit fluff and unrelated sections.",
			},
		},
	}

	req, err := NewExaRequest(ctx, "/search", data)
	if err != nil {
		return nil, err
	}

	return RunExaRequest(req)
}

func ExaRunContents(ctx context.Context, args FetchContentsArguments) (*ExaResults, error) {
	data := map[string]any{
		"urls": args.URLs,
		"text": map[string]any{
			"maxCharacters": 8000,
		},
	}

	req, err := NewExaRequest(ctx, "/contents", data)
	if err != nil {
		return nil, err
	}

	return RunExaRequest(req)
}
