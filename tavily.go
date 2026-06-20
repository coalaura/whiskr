package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

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
	Title      string              `json:"title"`
	Url        string              `json:"url"`
	Content    string              `json:"content"`
	Score      float64             `json:"score"`
	RawContent string              `json:"raw_content,omitempty"`
	Favicon    string              `json:"favicon,omitempty"`
	Images     []TavilySearchImage `json:"images,omitempty"`
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

type TavilyErrorResponse struct {
	Detail struct {
		Error string `json:"error"`
	} `json:"detail"`
}

func doTavilyRequest(path string, data, out any) error {
	var buf bytes.Buffer

	err := json.NewEncoder(&buf).Encode(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("https://api.tavily.com%s", path), &buf)
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
		var errResp TavilyErrorResponse

		err = json.NewDecoder(resp.Body).Decode(&errResp)
		if err == nil {
			return errors.New(errResp.Detail.Error)
		}

		return errors.New(resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

func DoTavilySearch(data TavilySearchRequest) (TavilySearchResponse, error) {
	var resp TavilySearchResponse

	err := doTavilyRequest("/search", data, &resp)
	return resp, err
}

func DoTavilyExtract(data TavilyExtractRequest) (TavilyExtractResponse, error) {
	var resp TavilyExtractResponse

	err := doTavilyRequest("/extract", data, &resp)
	return resp, err
}
