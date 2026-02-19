package main

import (
	"context"
	"slices"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/coalaura/openingrouter"
)

type ModelPricing struct {
	Input  float64 `json:"input"`
	Output float64 `json:"output"`
	Image  float64 `json:"image,omitzero"`
}

type Model struct {
	ID          string       `json:"id"`
	Created     int64        `json:"created"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Pricing     ModelPricing `json:"pricing"`
	Tags        []string     `json:"tags,omitempty"`

	Reasoning bool `json:"-"`
	Vision    bool `json:"-"`
	JSON      bool `json:"-"`
	Tools     bool `json:"-"`
	Images    bool `json:"-"`
	Audio     bool `json:"-"`
	Text      bool `json:"-"`
}

// Since there is no reliable image output pricing data :(
// These are for high quality, 1024x1024 output images
var ImageModelPricing = map[string]float64{
	"sourceful/riverflow-v2-pro":              0.15,  // https://openrouter.ai/sourceful/riverflow-v2-pro
	"sourceful/riverflow-v2-fast":             0.02,  // https://openrouter.ai/sourceful/riverflow-v2-fast
	"black-forest-labs/flux.2-klein-4b":       0.014, // https://openrouter.ai/black-forest-labs/flux.2-klein-4b
	"black-forest-labs/flux.2-max":            0.03,  // https://openrouter.ai/black-forest-labs/flux.2-max
	"sourceful/riverflow-v2-max-preview":      0.075, // https://openrouter.ai/sourceful/riverflow-v2-max-preview
	"sourceful/riverflow-v2-standard-preview": 0.035, // https://openrouter.ai/sourceful/riverflow-v2-standard-preview
	"sourceful/riverflow-v2-fast-preview":     0.03,  // https://openrouter.ai/sourceful/riverflow-v2-fast-preview
	"black-forest-labs/flux.2-flex":           0.06,  // https://openrouter.ai/black-forest-labs/flux.2-flex
	"black-forest-labs/flux.2-pro":            0.015, // https://openrouter.ai/black-forest-labs/flux.2-pro
	"google/gemini-3-pro-image-preview":       0.134, // https://ai.google.dev/gemini-api/docs/pricing#gemini-3-pro-image-preview
	"openai/gpt-5-image-mini":                 0.167, // https://developers.openai.com/api/docs/pricing/#image-generation
	"openai/gpt-5-image":                      0.036, // https://developers.openai.com/api/docs/pricing/#image-generation
	"google/gemini-2.5-flash-image":           0.039, // https://ai.google.dev/gemini-api/docs/pricing#gemini-2.5-flash-image
}

var (
	modelMx sync.RWMutex

	ModelMap  map[string]*Model
	ModelList []*Model
)

func GetModel(name string) *Model {
	modelMx.RLock()
	defer modelMx.RUnlock()

	return ModelMap[name]
}

func StartModelUpdateLoop() error {
	if err := LoadModels(); err != nil {
		return err
	}

	go func() {
		ticker := time.NewTicker(time.Duration(env.Settings.RefreshInterval) * time.Minute)

		for range ticker.C {
			if err := LoadModels(); err != nil {
				log.Warnln(err)
			}
		}
	}()

	return nil
}

func LoadModels() error {
	log.Println("Refreshing model list...")

	base, err := OpenRouterListModels(context.Background())
	if err != nil {
		return err
	}

	list, err := openingrouter.ListFrontendModels(context.Background())
	if err != nil {
		return err
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt.Time)
	})

	var (
		newList = make([]*Model, 0, len(list))
		newMap  = make(map[string]*Model, len(list))
	)

	for _, model := range list {
		if slices.Contains(model.OutputModalities, "embeddings") {
			continue
		}

		if model.Endpoint == nil {
			continue
		}

		var (
			input  float64
			output float64
		)

		if full, ok := base[model.Slug]; ok {
			input, _ = strconv.ParseFloat(full.Pricing.Prompt, 64)
			output, _ = strconv.ParseFloat(full.Pricing.Completion, 64)
		} else {
			input = model.Endpoint.Pricing.Prompt.Float64()
			output = model.Endpoint.Pricing.Completion.Float64()
		}

		m := &Model{
			ID:          model.Slug,
			Created:     model.CreatedAt.Unix(),
			Name:        model.ShortName,
			Description: model.Description,

			Pricing: ModelPricing{
				Input:  input * 1000000,
				Output: output * 1000000,
				Image:  ImageModelPricing[model.Slug],
			},
		}

		GetModelTags(model, m)

		if env.Models.filters != nil {
			matched, err := env.Models.filters.Match(m)
			if err != nil {
				return err
			}

			if !matched {
				continue
			}
		}

		newList = append(newList, m)
		newMap[m.ID] = m
	}

	log.Printf("Loaded %d models\n", len(newList))

	modelMx.Lock()

	ModelList = newList
	ModelMap = newMap

	modelMx.Unlock()

	return nil
}

func GetModelTags(model openingrouter.FrontendModel, m *Model) {
	for _, parameter := range model.Endpoint.SupportedParameters {
		switch parameter {
		case "reasoning":
			m.Reasoning = true

			m.Tags = append(m.Tags, "reasoning")
		case "response_format":
			m.JSON = true

			m.Tags = append(m.Tags, "json")
		case "tools":
			m.Tools = true

			m.Tags = append(m.Tags, "tools")
		}
	}

	for _, modality := range model.InputModalities {
		if modality == "image" {
			m.Vision = true

			m.Tags = append(m.Tags, "vision")
		}
	}

	for _, modality := range model.OutputModalities {
		switch modality {
		case "image":
			m.Images = true

			m.Tags = append(m.Tags, "image")
		case "audio":
			m.Audio = true

			m.Tags = append(m.Tags, "audio")
		case "text":
			m.Text = true
		}
	}

	if model.Endpoint.IsFree {
		m.Tags = append(m.Tags, "free")
	}

	sort.Strings(m.Tags)
}

func HasModelListChanged(list []openingrouter.FrontendModel) bool {
	modelMx.RLock()
	defer modelMx.RUnlock()

	if len(list) != len(ModelList) {
		return true
	}

	for i, model := range list {
		if ModelList[i].ID != model.Slug {
			return true
		}
	}

	return false
}
