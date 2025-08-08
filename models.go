package main

import (
	"context"
	"strings"
)

type Model struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	SupportedParameters []string `json:"supported_parameters,omitempty"`
}

var ModelMap = make(map[string]*Model)

func LoadModels() ([]*Model, error) {
	client := OpenRouterClient()

	list, err := client.ListUserModels(context.Background())
	if err != nil {
		return nil, err
	}

	models := make([]*Model, len(list))

	for index, model := range list {
		name := model.Name

		if index := strings.Index(name, ": "); index != -1 {
			name = name[index+2:]
		}

		m := &Model{
			ID:                  model.ID,
			Name:                name,
			Description:         model.Description,
			SupportedParameters: model.SupportedParameters,
		}

		models[index] = m

		ModelMap[model.ID] = m
	}

	return models, nil
}
