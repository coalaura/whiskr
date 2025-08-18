package main

import (
	"context"
	"sort"
	"strings"

	"github.com/revrost/go-openrouter"
)

type Model struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`

	Reasoning bool `json:"-"`
	Vision    bool `json:"-"`
	JSON      bool `json:"-"`
	Tools     bool `json:"-"`
}

var ModelMap = make(map[string]*Model)

func LoadModels() ([]*Model, error) {
	log.Info("Loading models...")

	client := OpenRouterClient()

	list, err := client.ListUserModels(context.Background())
	if err != nil {
		return nil, err
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].Created > list[j].Created
	})

	models := make([]*Model, len(list))

	for index, model := range list {
		name := model.Name

		if index := strings.Index(name, ": "); index != -1 {
			name = name[index+2:]
		}

		m := &Model{
			ID:          model.ID,
			Name:        name,
			Description: model.Description,
		}

		GetModelTags(model, m)

		models[index] = m

		ModelMap[model.ID] = m
	}

	log.Infof("Loaded %d models\n", len(models))

	return models, nil
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

	sort.Strings(m.Tags)
}
