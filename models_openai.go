package main

import (
	"context"
	"sort"

	"github.com/coalaura/openingrouter"
)

func LoadOpenAIModels() error {
	base, err := OpenRouterListModels(context.Background())
	if err != nil {
		return err
	}

	var (
		newModelList  = make([]*Model, 0, len(base))
		newModelMap   = make(map[string]*Model, len(base))
		newModelIDMap = make(map[string]*Model, len(base))
	)

	for _, model := range base {
		m := &Model{
			ID:          GetModelShortID(model.ID),
			Slug:        model.ID,
			Created:     model.Created,
			Name:        model.Name,
			Description: model.Description,

			Pricing: ModelPricing{
				Input:        model.Pricing.Prompt.Float64() * 1000000,
				Output:       model.Pricing.Completion.Float64() * 1000000,
				CacheRead:    model.Pricing.InputCacheRead.Float64() * 1000000,
				CacheWrite:   model.Pricing.InputCacheWrite.Float64() * 1000000,
				CacheWrite1H: model.Pricing.InputCacheWrite1H.Float64() * 1000000,
			},
			Context: ModelContext{
				Total:      openAIContextTotal(model),
				Completion: openAIInt(model.TopProvider.MaxCompletionTokens),
			},

			Benchmarks: GetModelBenchmarks(model),

			Text: true,
		}

		SetOpenAITags(model, m)

		if env.Models.filters != nil {
			matched, err := env.Models.filters.Match(m)
			if err != nil {
				return err
			}

			if !matched {
				continue
			}
		}

		newModelList = append(newModelList, m)
		newModelMap[model.ID] = m
		newModelIDMap[m.ID] = m
	}

	log.Printf("Loaded %d models\n", len(newModelList))

	modelMx.Lock()

	AudioList = nil
	ModelList = newModelList
	ModelMap = newModelMap
	ModelIDMap = newModelIDMap

	modelMx.Unlock()

	if settings != nil && settings.MigrateFavoriteModelIDs(newModelList) {
		settings.ScheduleStore()
	}

	return nil
}

func SetOpenAITags(model openingrouter.Model, m *Model) {
	for _, parameter := range model.SupportedParameters {
		switch parameter {
		case openingrouter.ParameterReasoning:
			m.Reasoning = true

			if model.Reasoning != nil && len(model.Reasoning.SupportedEfforts) > 0 {
				levels := make([]string, 0, len(model.Reasoning.SupportedEfforts))

				for _, effort := range model.Reasoning.SupportedEfforts {
					levels = append(levels, string(effort))
				}

				m.ReasoningLevels = levels
			}

			m.Tags = append(m.Tags, "reasoning")
		case openingrouter.ParameterResponseFormat:
			m.JSON = true

			m.Tags = append(m.Tags, "json")
		case openingrouter.ParameterTools:
			m.Tools = true

			m.Tags = append(m.Tags, "tools")
		}
	}

	for _, modality := range model.Architecture.InputModalities {
		if modality == openingrouter.InputModalityImage {
			m.Vision = true

			m.Tags = append(m.Tags, "vision")
		}
	}

	for _, modality := range model.Architecture.OutputModalities {
		switch modality {
		case openingrouter.OutputModalityImage:
			m.Images = true

			m.Tags = append(m.Tags, "image_gen")
		case openingrouter.OutputModalityText:
			m.Text = true
		case openingrouter.OutputModalityAudio:
			m.Audio = true
		}
	}

	if model.Pricing.Prompt.Float64() == 0 && model.Pricing.Completion.Float64() == 0 {
		m.Tags = append(m.Tags, "free")
	}

	sort.Strings(m.Tags)
}

func openAIInt(value *int) int {
	if value == nil {
		return 0
	}

	return *value
}

func openAIContextTotal(model openingrouter.Model) int {
	if model.ContextLength != nil {
		return *model.ContextLength
	}

	return openAIInt(model.TopProvider.ContextLength)
}
