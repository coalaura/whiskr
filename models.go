package main

import (
	"context"
	"slices"
	"sort"
	"strconv"
	"sync"
	"time"
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
	Providers   int64        `json:"providers"`

	Reasoning bool `json:"-"`
	Vision    bool `json:"-"`
	JSON      bool `json:"-"`
	Tools     bool `json:"-"`
	Images    bool `json:"-"`
	Audio     bool `json:"-"`
	Text      bool `json:"-"`
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

	list, err := OpenRouterListFrontendModels(context.Background())
	if err != nil {
		return err
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
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
			inputStr  string
			outputStr string
			imageStr  string
		)

		if full, ok := base[model.Slug]; ok {
			inputStr = full.Pricing.Prompt
			outputStr = full.Pricing.Completion
			imageStr = full.Pricing.Image
		} else {
			inputStr = model.Endpoint.Pricing.Prompt
			outputStr = model.Endpoint.Pricing.Completion
			imageStr = model.Endpoint.Pricing.Image
		}

		input, _ := strconv.ParseFloat(inputStr, 64)
		output, _ := strconv.ParseFloat(outputStr, 64)
		image, _ := strconv.ParseFloat(imageStr, 64)

		m := &Model{
			ID:          model.Slug,
			Created:     model.CreatedAt.Unix(),
			Name:        model.ShortName,
			Description: model.Description,

			Pricing: ModelPricing{
				Input:  input * 1000000,
				Output: output * 1000000,
				Image:  image,
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

func GetModelTags(model FrontendModel, m *Model) {
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

func HasModelListChanged(list []FrontendModel) bool {
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
