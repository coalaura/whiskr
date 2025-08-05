package main

import "context"

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
		m := &Model{
			ID:                  model.ID,
			Name:                model.Name,
			Description:         model.Description,
			SupportedParameters: model.SupportedParameters,
		}

		models[index] = m

		ModelMap[model.ID] = m
	}

	return models, nil
}
