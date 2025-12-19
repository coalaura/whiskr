package main

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/revrost/go-openrouter"
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
	err := LoadModels(true)
	if err != nil {
		return err
	}

	go func() {
		ticker := time.NewTicker(time.Duration(env.Settings.RefreshInterval) * time.Minute)

		for range ticker.C {
			err := LoadModels(false)
			if err != nil {
				log.Warnln(err)
			}
		}
	}()

	return nil
}

func LoadModels(initial bool) error {
	log.Println("Refreshing model list...")

	client := OpenRouterClient()

	list, err := client.ListUserModels(context.Background())
	if err != nil {
		return err
	}

	if !initial && !HasModelListChanged(list) {
		log.Println("No new models, skipping update")

		return nil
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].Created > list[j].Created
	})

	var (
		newList = make([]*Model, 0, len(list))
		newMap  = make(map[string]*Model, len(list))
	)

	for _, model := range list {
		name := model.Name

		if index := strings.Index(name, ": "); index != -1 {
			name = name[index+2:]
		}

		input, _ := strconv.ParseFloat(model.Pricing.Prompt, 64)
		output, _ := strconv.ParseFloat(model.Pricing.Completion, 64)
		image, _ := strconv.ParseFloat(model.Pricing.Image, 64)

		m := &Model{
			ID:          model.ID,
			Created:     model.Created,
			Name:        name,
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
		newMap[model.ID] = m
	}

	log.Printf("Loaded %d models\n", len(newList))

	modelMx.Lock()

	ModelList = newList
	ModelMap = newMap

	modelMx.Unlock()

	return nil
}

func GetModelTags(model openrouter.Model, m *Model) {
	for _, parameter := range model.SupportedParameters {
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

	for _, modality := range model.Architecture.InputModalities {
		if modality == "image" {
			m.Vision = true

			m.Tags = append(m.Tags, "vision")
		}
	}

	for _, modality := range model.Architecture.OutputModalities {
		if modality == "image" {
			m.Images = true

			m.Tags = append(m.Tags, "image")
		}
	}

	if model.Pricing.Prompt == "0" && model.Pricing.Completion == "0" {
		m.Tags = append(m.Tags, "free")
	}

	sort.Strings(m.Tags)
}

func HasModelListChanged(list []openrouter.Model) bool {
	modelMx.RLock()
	defer modelMx.RUnlock()

	if len(list) != len(ModelList) {
		return true
	}

	for i, model := range list {
		if ModelList[i].ID != model.ID {
			return true
		}
	}

	return false
}
