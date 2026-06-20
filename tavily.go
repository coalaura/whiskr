package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
)

type TavilyResult struct {
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	PublishedDate string  `json:"publishedDate,omitempty"`
	SiteName      string  `json:"siteName,omitempty"`
	Score         float64 `json:"score,omitempty"`
	Summary       string  `json:"summary,omitempty"`
	Text          string  `json:"text,omitempty"`
}

type TavilyResults struct {
	RequestID string         `json:"requestId,omitempty"`
	Query     string         `json:"query,omitempty"`
	Results   []TavilyResult `json:"results"`
	Usage     TavilyUsage    `json:"usage,omitempty"`
}

func (t *TavilyResults) String() string {
	buf := GetFreeBuffer()
	defer pool.Put(buf)

	json.NewEncoder(buf).Encode(map[string]any{
		"results": t.Results,
	})

	return buf.String()
}

type TavilyExtractRequest struct {
	Urls            any     `json:"urls"`
	Query           string  `json:"query,omitempty"`
	ExtractDepth    string  `json:"extract_depth,omitempty"`
	Format          string  `json:"format,omitempty"`
	ChunksPerSource int     `json:"chunks_per_source,omitempty"`
	Timeout         float64 `json:"timeout,omitempty"`
	IncludeImages   bool    `json:"include_images,omitempty"`
	IncludeFavicon  bool    `json:"include_favicon,omitempty"`
	IncludeUsage    bool    `json:"include_usage,omitempty"`
}

type TavilyExtractResult struct {
	Url        string   `json:"url"`
	RawContent string   `json:"raw_content"`
	Images     []string `json:"images,omitempty"`
	Favicon    string   `json:"favicon,omitempty"`
}

type TavilyExtractFailedResult struct {
	Url   string `json:"url"`
	Error string `json:"error"`
}

type TavilyUsage struct {
	Credits int `json:"credits"`
}

type TavilyExtractResponse struct {
	Results       []TavilyExtractResult       `json:"results"`
	FailedResults []TavilyExtractFailedResult `json:"failed_results"`
	ResponseTime  float64                     `json:"response_time"`
	Usage         TavilyUsage                 `json:"usage,omitempty"`
	RequestID     string                      `json:"request_id,omitempty"`
}

type TavilySearchRequest struct {
	IncludeDomains           []string `json:"include_domains,omitempty"`
	ExcludeDomains           []string `json:"exclude_domains,omitempty"`
	Query                    string   `json:"query"`
	SearchDepth              string   `json:"search_depth,omitempty"`
	Topic                    string   `json:"topic,omitempty"`
	TimeRange                string   `json:"time_range,omitempty"`
	StartDate                string   `json:"start_date,omitempty"`
	EndDate                  string   `json:"end_date,omitempty"`
	IncludeAnswer            any      `json:"include_answer,omitempty"`
	IncludeRawContent        any      `json:"include_raw_content,omitempty"`
	Country                  string   `json:"country,omitempty"`
	ChunksPerSource          int      `json:"chunks_per_source,omitempty"`
	MaxResults               int      `json:"max_results,omitempty"`
	IncludeImages            bool     `json:"include_images,omitempty"`
	IncludeImageDescriptions bool     `json:"include_image_descriptions,omitempty"`
	IncludeFavicon           bool     `json:"include_favicon,omitempty"`
	AutoParameters           bool     `json:"auto_parameters,omitempty"`
	ExactMatch               bool     `json:"exact_match,omitempty"`
	IncludeUsage             bool     `json:"include_usage,omitempty"`
	SafeSearch               bool     `json:"safe_search,omitempty"`
}

type TavilySearchImage struct {
	Url         string `json:"url"`
	Description string `json:"description,omitempty"`
}

type TavilySearchResult struct {
	Title         string              `json:"title"`
	Url           string              `json:"url"`
	Content       string              `json:"content"`
	Score         float64             `json:"score"`
	RawContent    string              `json:"raw_content,omitempty"`
	PublishedDate string              `json:"published_date,omitempty"`
	Favicon       string              `json:"favicon,omitempty"`
	Images        []TavilySearchImage `json:"images,omitempty"`
}

type TavilySearchResponse struct {
	Query          string               `json:"query"`
	Answer         string               `json:"answer,omitempty"`
	Images         []TavilySearchImage  `json:"images"`
	Results        []TavilySearchResult `json:"results"`
	AutoParameters any                  `json:"auto_parameters,omitempty"`
	ResponseTime   float64              `json:"response_time"`
	Usage          TavilyUsage          `json:"usage,omitempty"`
	RequestID      string               `json:"request_id,omitempty"`
}

type TavilyQueryResult struct {
	Resp TavilySearchResponse
	Err  error
}

func doTavilyRequest(ctx context.Context, path string, data, out any) error {
	var buf bytes.Buffer

	err := json.NewEncoder(&buf).Encode(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("https://api.tavily.com%s", path), &buf)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", env.Tokens.Tavily))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("tavily api error (%d): %v", resp.StatusCode, err)
		}

		return fmt.Errorf("tavily api error (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

func DoTavilySearch(ctx context.Context, data TavilySearchRequest) (TavilySearchResponse, error) {
	var resp TavilySearchResponse

	err := doTavilyRequest(ctx, "/search", data, &resp)
	return resp, err
}

func DoTavilyExtract(ctx context.Context, data TavilyExtractRequest) (TavilyExtractResponse, error) {
	var resp TavilyExtractResponse

	err := doTavilyRequest(ctx, "/extract", data, &resp)
	return resp, err
}

func TavilyRunSearch(ctx context.Context, args *SearchWebArguments) (*TavilyResults, error) {
	queries := make([]string, 0, len(args.Queries))
	for _, q := range args.Queries {
		if q = strings.TrimSpace(q); q != "" {
			queries = append(queries, q)
		}
	}

	if len(queries) == 0 {
		return nil, fmt.Errorf("no search query")
	}

	if len(queries) > 5 {
		queries = queries[:5]
	}

	if args.MaxResults <= 0 {
		args.MaxResults = 5
	} else if args.MaxResults > 20 {
		args.MaxResults = 20
	}

	var (
		searchDepth     = "fast"
		chunksPerSource int
	)

	if args.Depth == "thorough" {
		searchDepth = "advanced"
		chunksPerSource = 3
	}

	var (
		wg      sync.WaitGroup
		outputs = make([]TavilyQueryResult, len(queries))
	)

	for i, query := range queries {
		wg.Go(func() {
			req := TavilySearchRequest{
				Query:           query,
				MaxResults:      args.MaxResults,
				Topic:           args.Topic,
				TimeRange:       args.TimeRange,
				StartDate:       args.StartDate,
				EndDate:         args.EndDate,
				IncludeDomains:  args.IncludeDomains,
				ExcludeDomains:  args.ExcludeDomains,
				SearchDepth:     searchDepth,
				ChunksPerSource: chunksPerSource,
				IncludeUsage:    true,
			}

			resp, err := DoTavilySearch(ctx, req)

			outputs[i] = TavilyQueryResult{
				Resp: resp,
				Err:  err,
			}
		})
	}

	wg.Wait()

	results := &TavilyResults{
		Query:   strings.Join(queries, " | "),
		Results: make([]TavilyResult, 0),
	}

	var (
		firstErr error
		seen     = make(map[string]int)
	)

	for _, result := range outputs {
		if result.Err != nil {
			if firstErr == nil {
				firstErr = result.Err
			}

			continue
		}

		if results.RequestID == "" {
			results.RequestID = result.Resp.RequestID
		}

		results.Usage.Credits += result.Resp.Usage.Credits

		for _, result := range result.Resp.Results {
			converted := TavilyResult{
				Title:         result.Title,
				URL:           result.Url,
				PublishedDate: result.PublishedDate,
				Score:         result.Score,
				Summary:       result.Content,
				Text:          result.RawContent,
			}

			if idx, ok := seen[result.Url]; ok {
				if converted.Score > results.Results[idx].Score {
					results.Results[idx] = converted
				}

				continue
			}

			seen[result.Url] = len(results.Results)
			results.Results = append(results.Results, converted)
		}
	}

	if len(results.Results) == 0 && firstErr != nil {
		return nil, firstErr
	}

	sort.SliceStable(results.Results, func(i, j int) bool {
		return results.Results[i].Score > results.Results[j].Score
	})

	return results, nil
}

func TavilyRunContents(ctx context.Context, args *FetchContentsArguments) (*TavilyResults, error) {
	resp, err := DoTavilyExtract(ctx, TavilyExtractRequest{
		Urls:         args.URLs,
		ExtractDepth: "advanced",
		Format:       "markdown",
		IncludeUsage: true,
	})
	if err != nil {
		return nil, err
	}

	if len(resp.Results) == 0 && len(resp.FailedResults) > 0 {
		return nil, fmt.Errorf("failed to fetch contents: %s", resp.FailedResults[0].Error)
	}

	results := &TavilyResults{
		RequestID: resp.RequestID,
		Usage:     resp.Usage,
		Results:   make([]TavilyResult, 0, len(resp.Results)),
	}

	for _, result := range resp.Results {
		results.Results = append(results.Results, TavilyResult{
			URL:  result.Url,
			Text: result.RawContent,
		})
	}

	return results, nil
}
