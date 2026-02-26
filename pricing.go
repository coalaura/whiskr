package main

type ImagePricing struct {
	K1 float64 `json:"k_1"`
	K2 float64 `json:"k_2,omitempty"`
	K4 float64 `json:"k_4,omitempty"`
}

func NewImagePricing(k ...float64) *ImagePricing {
	if len(k) < 1 || len(k) > 3 {
		return nil
	}

	var (
		k1 = k[0]
		k2 float64
		k4 float64
	)

	if len(k) > 1 {
		k2 = k[1]

		if len(k) > 2 {
			k4 = k[2]
		}
	}

	return &ImagePricing{
		K1: k1,
		K2: k2,
		K4: k4,
	}
}

// Since there is no reliable image output pricing data :(
var ImageModelPricing = map[string]*ImagePricing{
	// https://ai.google.dev/gemini-api/docs/pricing#gemini-3.1-flash-image-preview
	"google/gemini-3.1-flash-image-preview": NewImagePricing(0.067, 0.101, 0.151),

	// https://openrouter.ai/sourceful/riverflow-v2-pro
	"sourceful/riverflow-v2-pro": NewImagePricing(0.15, 0.15, 0.33),

	// https://openrouter.ai/sourceful/riverflow-v2-fast
	"sourceful/riverflow-v2-fast": NewImagePricing(0.02, 0.04), // No 4K support

	// https://openrouter.ai/black-forest-labs/flux.2-klein-4b
	"black-forest-labs/flux.2-klein-4b": NewImagePricing(0.014, 0.015, 0.016),

	// https://openrouter.ai/black-forest-labs/flux.2-max
	"black-forest-labs/flux.2-max": NewImagePricing(0.07, 0.1, 0.13),

	// https://openrouter.ai/sourceful/riverflow-v2-max-preview
	"sourceful/riverflow-v2-max-preview": NewImagePricing(0.075, 0.075, 0.075),

	// https://openrouter.ai/sourceful/riverflow-v2-standard-preview
	"sourceful/riverflow-v2-standard-preview": NewImagePricing(0.035, 0.035, 0.035),

	// https://openrouter.ai/sourceful/riverflow-v2-fast-preview
	"sourceful/riverflow-v2-fast-preview": NewImagePricing(0.03, 0.03, 0.03),

	// https://openrouter.ai/black-forest-labs/flux.2-flex
	"black-forest-labs/flux.2-flex": NewImagePricing(0.06, 0.12, 0.18),

	// https://openrouter.ai/black-forest-labs/flux.2-pro
	"black-forest-labs/flux.2-pro": NewImagePricing(0.03, 0.045, 0.06),

	// https://ai.google.dev/gemini-api/docs/pricing#gemini-3-pro-image-preview
	"google/gemini-3-pro-image-preview": NewImagePricing(0.134, 0.134, 0.24),

	// https://developers.openai.com/api/docs/pricing/#image-generation
	"openai/gpt-5-image-mini": NewImagePricing(0.036, 0.052), // No "true" 2K support, no 4K support

	// https://developers.openai.com/api/docs/pricing/#image-generation
	"openai/gpt-5-image": NewImagePricing(0.167, 0.25), // No "true" 2K support, no 4K support

	// https://ai.google.dev/gemini-api/docs/pricing#gemini-2.5-flash-image
	"google/gemini-2.5-flash-image": NewImagePricing(0.039), // No 2K or 4K support
}
