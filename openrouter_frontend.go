package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/revrost/go-openrouter"
)

type FrontendModelsResponse struct {
	Data []FrontendModel `json:"data"`
}

type FrontendModel struct {
	Slug             string    `json:"slug"`
	Name             string    `json:"name"`
	ShortName        string    `json:"short_name"`
	Description      string    `json:"description"`
	CreatedAt        time.Time `json:"created_at"`
	InputModalities  []string  `json:"input_modalities"`
	OutputModalities []string  `json:"output_modalities"`
	Endpoint         *Endpoint `json:"endpoint"`
}

type Endpoint struct {
	ID                  string   `json:"id"`
	IsFree              bool     `json:"is_free"`
	SupportedParameters []string `json:"supported_parameters"`
	Pricing             Pricing  `json:"pricing"`
}

type Pricing struct {
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
	Image      string `json:"image,omitempty"`
}

func OpenRouterListFrontendModels(ctx context.Context) ([]FrontendModel, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://openrouter.ai/api/frontend/models", nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var result FrontendModelsResponse

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	return result.Data, nil
}

func OpenRouterListModels(ctx context.Context) (map[string]openrouter.Model, error) {
	client := OpenRouterClient()

	models, err := client.ListModels(ctx)
	if err != nil {
		return nil, err
	}

	mp := make(map[string]openrouter.Model, len(models))

	for _, model := range models {
		mp[model.ID] = model
	}

	return mp, nil
}
