package main

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
)

func (s *Settings) SetPreference(username, name string, data json.RawMessage) error {
	s.mx.Lock()
	defer s.mx.Unlock()

	user := s.getLocked(username)

	var err error

	switch name {
	case "cost_limit":
		err = decodeCostLimitPreference(data, &user.CostLimit)
	case "retry_max_retries":
		err = decodeIntegerPreference(data, &user.RetryMaxRetries, 0, 100)
	case "retry_initial_delay":
		err = decodeIntegerPreference(data, &user.RetryInitialDelay, 1, 3600)
	case "retry_max_delay":
		err = decodeIntegerPreference(data, &user.RetryMaxDelay, 1, 3600)
	case "tts_model":
		err = decodeStringPreference(data, &user.TTSModel)
	case "tts_voice":
		err = decodeStringPreference(data, &user.TTSVoice)
	case "theme":
		err = decodeStringPreference(data, &user.Theme, "catppuccin", "rose-pine", "gruvbox", "tokyo-night", "dracula", "nord", "one-dark", "kanagawa", "poimandres", "everforest", "night-owl", "andromeda", "aura", "vitesse", "synthwave-84")
	case "provider_sorting":
		err = decodeStringPreference(data, &user.ProviderSorting, "", "throughput", "latency", "price")
	case "model_benchmark":
		err = decodeStringPreference(data, &user.ModelBenchmark, "none", "intelligence", "coding", "agentic")
	case "image_resize":
		err = decodeStringPreference(data, &user.ImageResize, "0", "512", "1024", "1536", "2048", "4096", "8192")
	case "max_images":
		err = decodeIntegerPreference(data, &user.MaxImages, 0, 9007199254740991)
	default:
		return errors.New("unknown setting")
	}

	if err != nil {
		return err
	}

	s.ScheduleStore()

	return nil
}

func decodeIntegerPreference(data json.RawMessage, target **int64, minimum, maximum int64) error {
	var value *int64

	err := json.Unmarshal(data, &value)
	if err != nil {
		return err
	}

	if value == nil || *value < minimum || *value > maximum {
		return errors.New("setting value is out of range")
	}

	*target = value

	return nil
}

func decodeCostLimitPreference(data json.RawMessage, target **float64) error {
	var value *float64

	err := json.Unmarshal(data, &value)
	if err != nil {
		return err
	}

	if value == nil || !validCostLimit(*value) {
		return errors.New("cost limit must be a positive finite number")
	}

	*target = value

	return nil
}

func decodeStringPreference(data json.RawMessage, target **string, choices ...string) error {
	var value *string

	err := json.Unmarshal(data, &value)
	if err != nil {
		return err
	}

	if value == nil {
		return errors.New("setting value must be a string")
	}

	if len(choices) > 0 {
		if !slices.Contains(choices, *value) {
			return errors.New("invalid setting choice")
		}
	} else if *value == "" || len(*value) > 256 || strings.TrimSpace(*value) != *value {
		return errors.New("invalid voice selection")
	}

	*target = value

	return nil
}
