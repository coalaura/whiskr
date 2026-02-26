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
	Input  float64       `json:"input"`
	Output float64       `json:"output"`
	Image  *ImagePricing `json:"image,omitempty"`
}

type Model struct {
	ID          string       `json:"id"`
	Created     int64        `json:"created"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Pricing     ModelPricing `json:"pricing"`
	Tags        []string     `json:"tags,omitempty"`
	Author      string       `json:"author,omitempty"`

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
			Author:      model.Author,

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

			m.Tags = append(m.Tags, "image_gen")
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
