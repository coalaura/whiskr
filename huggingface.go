package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-yaml"
)

type HuggingFaceMetadata struct {
	Quantization    map[string]any    `yaml:"quantization"`
	License         string            `yaml:"license"`
	Parameters      map[string]uint64 `yaml:"parameters"`
	TotalParameters uint64            `yaml:"total_parameters"`
}

type HuggingFaceCache struct {
	mx     sync.Mutex
	client *http.Client

	token    string
	filename string

	models  map[string]*HuggingFaceMetadata
	pending map[string]*huggingFaceRequest
}

type huggingFaceRequest struct {
	done     chan struct{}
	metadata *HuggingFaceMetadata
	err      error
}

type huggingFaceResponse struct {
	Config      huggingFaceConfig      `json:"config"`
	CardData    huggingFaceCardData    `json:"cardData"`
	Safetensors huggingFaceSafetensors `json:"safetensors"`
}

type huggingFaceConfig struct {
	Quantization map[string]any     `json:"quantization_config"`
	TextConfig   *huggingFaceConfig `json:"text_config"`
}

type huggingFaceCardData struct {
	License string `json:"license"`
}

type huggingFaceSafetensors struct {
	Parameters map[string]uint64 `json:"parameters"`
	Total      uint64            `json:"total"`
}

func (cache *HuggingFaceCache) Get(ctx context.Context, slug string) (*HuggingFaceMetadata, error) {
	if !validHuggingFaceSlug(slug) {
		return nil, nil
	}

	cache.mx.Lock()

	err := cache.loadLocked()
	if err != nil {
		cache.mx.Unlock()

		return nil, err
	}

	if metadata, ok := cache.models[slug]; ok {
		cache.mx.Unlock()

		return metadata, nil
	}

	if pending, ok := cache.pending[slug]; ok {
		cache.mx.Unlock()

		select {
		case <-pending.done:
			return pending.metadata, pending.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	pending := &huggingFaceRequest{done: make(chan struct{})}

	cache.pending[slug] = pending

	cache.mx.Unlock()

	metadata, err := cache.fetch(ctx, slug)

	cache.mx.Lock()
	defer cache.mx.Unlock()

	if err == nil {
		cache.models[slug] = metadata

		err = cache.storeLocked()
	}

	pending.metadata = metadata
	pending.err = err

	delete(cache.pending, slug)
	close(pending.done)

	return metadata, err
}

func (cache *HuggingFaceCache) loadLocked() error {
	if cache.models != nil {
		return nil
	}

	data, err := os.ReadFile(cache.filename)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	models := make(map[string]*HuggingFaceMetadata)

	if len(data) > 0 {
		err = yaml.Unmarshal(data, &models)
		if err != nil {
			return fmt.Errorf("load Hugging Face cache: %w", err)
		}
	}

	if models == nil {
		models = make(map[string]*HuggingFaceMetadata)
	}

	cache.models = models

	return nil
}

func (cache *HuggingFaceCache) storeLocked() error {
	file, err := os.CreateTemp(filepath.Dir(cache.filename), ".huggingface-*.yml")
	if err != nil {
		return err
	}

	defer os.Remove(file.Name())

	err = yaml.NewEncoder(file).Encode(cache.models)
	if err == nil {
		err = file.Sync()
	}

	closeErr := file.Close()

	if err != nil {
		return err
	}

	if closeErr != nil {
		return closeErr
	}

	return os.Rename(file.Name(), cache.filename)
}

func (cache *HuggingFaceCache) fetch(ctx context.Context, slug string) (*HuggingFaceMetadata, error) {
	endpoint := "https://huggingface.co/api/models/" + slug

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Accept", "application/json")

	if cache.token != "" {
		request.Header.Set("Authorization", "Bearer "+cache.token)
	}

	response, err := cache.client.Do(request)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hugging face metadata for %s: HTTP %d", slug, response.StatusCode)
	}

	var payload huggingFaceResponse

	err = json.NewDecoder(io.LimitReader(response.Body, 16<<20)).Decode(&payload)
	if err != nil {
		return nil, fmt.Errorf("hugging face metadata for %s: %w", slug, err)
	}

	quantization := payload.Config.Quantization
	if len(quantization) == 0 && payload.Config.TextConfig != nil {
		quantization = payload.Config.TextConfig.Quantization
	}

	return &HuggingFaceMetadata{
		Quantization:    quantization,
		License:         payload.CardData.License,
		Parameters:      payload.Safetensors.Parameters,
		TotalParameters: payload.Safetensors.Total,
	}, nil
}

func NewHuggingFaceCache(filename, token string) *HuggingFaceCache {
	return &HuggingFaceCache{
		filename: filename,
		token:    token,
		client:   &http.Client{Timeout: 10 * time.Second},
		pending:  make(map[string]*huggingFaceRequest),
	}
}

func validHuggingFaceSlug(slug string) bool {
	var parts int

	for part := range strings.SplitSeq(slug, "/") {
		parts++

		if parts > 2 || part == "" || part == "." || part == ".." {
			return false
		}

		for _, character := range part {
			letter := character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
			digit := character >= '0' && character <= '9'

			if !letter && !digit && character != '-' && character != '_' && character != '.' {
				return false
			}
		}
	}

	return true
}
