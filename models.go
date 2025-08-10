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
	JSON      bool `json:"-"`
}

var ModelMap = make(map[string]*Model)

func LoadModels() ([]*Model, error) {
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

		tags, reasoning, json := GetModelTags(model)

		m := &Model{
			ID:          model.ID,
			Name:        name,
			Description: model.Description,
			Tags:        tags,

			Reasoning: reasoning,
			JSON:      json,
		}

		models[index] = m

		ModelMap[model.ID] = m
	}

	return models, nil
}

func GetModelTags(model openrouter.Model) ([]string, bool, bool) {
	var (
		reasoning bool
		json      bool
		tags      []string
	)

	for _, parameter := range model.SupportedParameters {
		switch parameter {
		case "reasoning":
			reasoning = true

			tags = append(tags, "reasoning")
		case "response_format":
			json = true

			tags = append(tags, "json")
		case "tools":
			tags = append(tags, "tools")
		}
	}

	for _, modality := range model.Architecture.InputModalities {
		if modality == "image" {
			tags = append(tags, "vision")
		}
	}

	sort.Strings(tags)

	return tags, reasoning, json
}
