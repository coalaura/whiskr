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

		m := &Model{
			ID:          model.ID,
			Name:        name,
			Description: model.Description,
			Tags:        GetModelTags(model),
		}

		models[index] = m

		ModelMap[model.ID] = m
	}

	return models, nil
}

func GetModelTags(model openrouter.Model) []string {
	var tags []string

	for _, parameter := range model.SupportedParameters {
		if parameter == "reasoning" || parameter == "tools" {
			tags = append(tags, parameter)
		}
	}

	sort.Strings(tags)

	return tags
}
