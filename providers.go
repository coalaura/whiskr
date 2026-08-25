package main

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coalaura/openingrouter"
)

const providerCacheTTL = 4 * time.Hour

type ProviderInfo struct {
	Name          string
	DisplayName   string
	Icon          string
	Training      bool
	Retains       bool
	RetentionDays *int
}

type ModelProvider struct {
	Slug          string  `json:"slug"`
	Name          string  `json:"name"`
	Icon          string  `json:"icon,omitempty"`
	Input         float64 `json:"input"`
	Output        float64 `json:"output"`
	Discount      float64 `json:"discount,omitempty"`
	Training      bool    `json:"training"`
	Retains       bool    `json:"retains"`
	RetentionDays *int    `json:"retention_days,omitempty"`
}

type ProviderGroup struct {
	Name     string
	Input    float64
	Output   float64
	Discount float64
	HaveIn   bool
	HaveOut  bool
}

var (
	providerMx         sync.Mutex
	providerRegistry   map[string]ProviderInfo
	providerRegistryAt time.Time

	modelProviders   = map[string][]ModelProvider{}
	modelProvidersAt = map[string]time.Time{}
)

func GetModelProviders(ctx context.Context, slug string) ([]ModelProvider, error) {
	providerMx.Lock()

	if cached, ok := modelProviders[slug]; ok && time.Since(modelProvidersAt[slug]) < providerCacheTTL {
		providerMx.Unlock()

		return cached, nil
	}

	stale, staleOK := modelProviders[slug]

	providerMx.Unlock()

	registry, err := LoadProviderRegistry(ctx)
	if err != nil {
		if staleOK {
			return stale, nil
		}

		return nil, err
	}

	response, err := OpenRouterClient(nil).GetModelEndpoints(ctx, slug)
	if err != nil {
		if staleOK {
			return stale, nil
		}

		return nil, err
	}

	groups := make(map[string]*ProviderGroup)
	order := make([]string, 0)
	groupOrder := make(map[string]int)

	for _, endpoint := range response.Endpoints {
		providerSlug := endpoint.Tag

		if providerSlug == "" {
			continue
		}

		group, ok := groups[providerSlug]
		if !ok {
			group = &ProviderGroup{Name: endpoint.ProviderName}

			groups[providerSlug] = group
			order = append(order, providerSlug)

			baseSlug := baseProviderSlug(providerSlug)
			if _, exists := groupOrder[baseSlug]; !exists {
				groupOrder[baseSlug] = len(groupOrder)
			}
		}

		discount := endpoint.Pricing.Discount

		if discount > 0 && discount > group.Discount {
			group.Discount = discount
		}

		input := float64(endpoint.Pricing.Prompt) * 1000000
		output := float64(endpoint.Pricing.Completion) * 1000000

		if !group.HaveIn || input < group.Input {
			group.Input = input
			group.HaveIn = true
		}

		if !group.HaveOut || output < group.Output {
			group.Output = output
			group.HaveOut = true
		}
	}

	sort.SliceStable(order, func(left, right int) bool {
		leftGroup := groupOrder[baseProviderSlug(order[left])]
		rightGroup := groupOrder[baseProviderSlug(order[right])]

		return leftGroup < rightGroup
	})

	providers := make([]ModelProvider, 0, len(order))

	for _, providerSlug := range order {
		group := groups[providerSlug]
		baseSlug := baseProviderSlug(providerSlug)

		info, registered := registry[providerSlug]
		if !registered {
			info = registry[baseSlug]
		}

		name := info.DisplayName
		if name == "" {
			name = info.Name
		}

		if name == "" {
			name = group.Name
		}

		if name == "" {
			name = baseSlug
		}

		if !registered {
			name = providerVariantName(name, providerSlug)
		}

		providers = append(providers, ModelProvider{
			Slug:          providerSlug,
			Name:          name,
			Icon:          info.Icon,
			Input:         group.Input,
			Output:        group.Output,
			Discount:      group.Discount,
			Training:      info.Training,
			Retains:       info.Retains,
			RetentionDays: info.RetentionDays,
		})
	}

	providerMx.Lock()

	modelProviders[slug] = providers
	modelProvidersAt[slug] = time.Now()

	providerMx.Unlock()

	return providers, nil
}

func HandleModelProviders(w http.ResponseWriter, r *http.Request) {
	if env.IsOpenAI() {
		RespondJson(w, http.StatusOK, []ModelProvider{})

		return
	}

	slug := r.URL.Query().Get("model")

	if slug == "" {
		RespondJson(w, http.StatusOK, []ModelProvider{})

		return
	}

	providers, err := GetModelProviders(r.Context(), slug)
	if err != nil {
		log.Warnln(err)

		RespondJson(w, http.StatusOK, []ModelProvider{})

		return
	}

	RespondJson(w, http.StatusOK, providers)
}

func LoadProviderRegistry(ctx context.Context) (map[string]ProviderInfo, error) {
	providerMx.Lock()

	if providerRegistry != nil && time.Since(providerRegistryAt) < providerCacheTTL {
		registry := providerRegistry

		providerMx.Unlock()

		return registry, nil
	}

	providerMx.Unlock()

	providers, err := openingrouter.ListFrontendProviders(ctx)
	if err != nil {
		return nil, err
	}

	registry := make(map[string]ProviderInfo, len(providers))

	for _, provider := range providers {
		info := ProviderInfo{
			Name:        provider.Name,
			DisplayName: provider.DisplayName,
		}

		if provider.Icon != nil {
			uri, err := CacheProviderIcon(ctx, provider.Name, provider.Icon.URL)
			if err != nil {
				log.Warnf("Unable to cache icon for provider %q: %v\n", provider.Name, err)
			} else {
				info.Icon = uri
			}
		}

		if provider.DataPolicy != nil {
			info.Training = provider.DataPolicy.Training
			info.Retains = provider.DataPolicy.RetainsPrompts
			info.RetentionDays = provider.DataPolicy.RetentionDays
		}

		registry[provider.Slug] = info
	}

	providerMx.Lock()

	providerRegistry = registry
	providerRegistryAt = time.Now()

	providerMx.Unlock()

	return registry, nil
}

func baseProviderSlug(tag string) string {
	if before, _, ok := strings.Cut(tag, "/"); ok {
		return before
	}

	return tag
}

func providerVariantName(name, tag string) string {
	_, variant, ok := strings.Cut(tag, "/")
	if !ok || variant == "" {
		return name
	}

	variant = strings.NewReplacer("/", " ", "-", " ", "_", " ").Replace(variant)
	words := strings.Fields(variant)

	for index, word := range words {
		if len(word) <= 2 {
			words[index] = strings.ToUpper(word)

			continue
		}

		words[index] = strings.ToUpper(word[:1]) + word[1:]
	}

	return name + " (" + strings.Join(words, " ") + ")"
}
