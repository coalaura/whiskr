package main

import (
	"context"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coalaura/openingrouter"
	"github.com/revrost/go-openrouter"
)

type ModelPricing struct {
	Input  float64       `json:"input"`
	Output float64       `json:"output"`
	Image  *ImagePricing `json:"image,omitempty"`
}

type ModelBenchmarks struct {
	Intelligence float64 `json:"intelligence,omitempty"`
	Coding       float64 `json:"coding,omitempty"`
	Agentic      float64 `json:"agentic,omitempty"`
}

// gost:preserve-layout
type Model struct {
	ID          string           `json:"id"`
	Created     int64            `json:"created"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Pricing     ModelPricing     `json:"pricing"`
	Benchmarks  *ModelBenchmarks `json:"benchmarks,omitempty"`
	Tags        []string         `json:"tags,omitempty"`
	Author      string           `json:"author,omitempty"`

	Reasoning       bool     `json:"reasoning"`
	ReasoningLevels []string `json:"reasoning_levels,omitempty"`
	Voices          []string `json:"voices,omitempty"`

	IsRouter bool `json:"is_router"`

	Vision bool `json:"-"`
	JSON   bool `json:"-"`
	Tools  bool `json:"-"`
	Images bool `json:"-"`
	Audio  bool `json:"-"`
	Text   bool `json:"-"`
}

var (
	modelMx sync.RWMutex

	AudioList []*Model
	ModelList []*Model
	ModelMap  map[string]*Model
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
		newModelList = make([]*Model, 0, len(list))
		newAudioList = make([]*Model, 0, len(list))
		newModelMap  = make(map[string]*Model, len(list))
	)

	for _, model := range list {
		if model.Endpoint == nil {
			continue
		}

		canText := slices.Contains(model.OutputModalities, "text")
		canImage := env.Models.ImageGeneration && slices.Contains(model.OutputModalities, "image")
		canAudio := env.Models.TextToSpeech && slices.Contains(model.OutputModalities, "audio")

		if !canText && !canImage && !canAudio {
			continue
		}

		var (
			input  float64
			output float64

			benchmarks      *ModelBenchmarks
			supportedVoices []string
		)

		if full, ok := base[model.Slug]; ok {
			input, _ = strconv.ParseFloat(full.Pricing.Prompt, 64)
			output, _ = strconv.ParseFloat(full.Pricing.Completion, 64)

			benchmarks = GetModelBenchmarks(full)

			if full.SupportedVoices != nil {
				supportedVoices = *full.SupportedVoices
			}
		} else {
			input = model.Endpoint.Pricing.Prompt.Float64()
			output = model.Endpoint.Pricing.Completion.Float64()
		}

		m := &Model{
			ID:          model.Slug,
			Created:     model.CreatedAt.Unix(),
			Name:        CleanModelName(model.Author, model.ShortName),
			Description: model.Description,
			Author:      model.Author,

			Voices: supportedVoices,

			Benchmarks: benchmarks,
			Pricing: ModelPricing{
				Input:  input * 1000000,
				Output: output * 1000000,
				Image:  ImageModelPricing[model.Slug],
			},

			IsRouter: strings.EqualFold(model.Group, "router"),
		}

		GetModelTags(model, m)

		if canText || canImage {
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
		}

		if canAudio {
			newAudioList = append(newAudioList, m)
		}

		newModelMap[m.ID] = m
	}

	log.Printf("Loaded %d models\n", len(newModelList))

	modelMx.Lock()

	AudioList = newAudioList
	ModelList = newModelList
	ModelMap = newModelMap

	modelMx.Unlock()

	return nil
}

func GetModelBenchmarks(model openrouter.Model) *ModelBenchmarks {
	benchmarks := model.Benchmarks
	if benchmarks == nil {
		return nil
	}

	artificial := benchmarks.ArtificialAnalysis

	intelligence := artificial.IntelligenceIndex
	coding := artificial.CodingIndex
	agentic := artificial.AgenticIndex

	if intelligence == nil && coding == nil && agentic == nil {
		return nil
	}

	var result ModelBenchmarks

	if intelligence != nil {
		result.Intelligence = *intelligence
	}

	if coding != nil {
		result.Coding = *coding
	}

	if agentic != nil {
		result.Agentic = *agentic
	}

	return &result
}

func GetModelTags(model openingrouter.FrontendModel, m *Model) {
	for _, parameter := range model.Endpoint.SupportedParameters {
		switch parameter {
		case "reasoning":
			m.Reasoning = true

			reasoning := model.ReasoningConfig

			if reasoning != nil {
				m.ReasoningLevels = reasoning.SupportedReasoningEfforts
			}

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
		case "audio":
			m.Audio = true
		}
	}

	if model.Endpoint.IsFree {
		m.Tags = append(m.Tags, "free")
	}

	sort.Strings(m.Tags)
}

func CleanModelName(author, name string) string {
	if len(name) < len(author) {
		return name
	}

	if !strings.EqualFold(name[:len(author)], author) {
		return name
	}

	trimmed := strings.TrimSpace(name[len(author):])

	// Special case, we trimmed too much
	if len(trimmed) < 3 || HasVersionPrefix(trimmed) {
		return name
	}

	return trimmed
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

func HasVersionPrefix(str string) bool {
	ln := len(str)

	// "v123"
	if ln >= 2 && strings.EqualFold(str[:1], "v") && isDigit(str[1]) {
		return true
	}

	// "Mk123"
	if ln >= 3 && strings.EqualFold(str[:2], "mk") && isDigit(str[2]) {
		return true
	}

	// "1.2"
	return ln > 0 && isDigit(str[0])
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}
