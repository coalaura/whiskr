package main

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
)

type ModelMetadata struct {
	NativeQuantization string `json:"native_quantization"`
	HFSlug             string `json:"hf_slug,omitempty"`
}

func (metadata *HuggingFaceMetadata) NativeQuantization() string {
	if metadata == nil {
		return "unknown"
	}

	method, _ := metadata.Quantization["quant_method"].(string)
	method = strings.ToUpper(strings.TrimSpace(method))

	if method == "COMPRESSED-TENSORS" {
		format := compressedTensorQuantization(metadata.Quantization)
		if format != "" {
			return format
		}
	}

	if method != "" {
		if bits, ok := metadata.Quantization["bits"]; ok {
			return fmt.Sprintf("%s (%v-bit)", method, bits)
		}

		return method
	}

	// Integer storage can contain packed weights; it does not identify their precision.
	if len(metadata.Quantization) > 0 {
		return "unknown"
	}

	formats := make([]string, 0, len(metadata.Parameters))

	for dtype, count := range metadata.Parameters {
		if count == 0 {
			continue
		}

		format := ""

		switch dtype {
		case "BF16":
			format = "BF16"
		case "F16":
			format = "FP16"
		case "F32":
			format = "FP32"
		case "F64":
			format = "FP64"
		case "F8_E4M3", "F8_E5M2":
			format = "FP8"
		default:
			return "unknown"
		}

		if !slices.Contains(formats, format) {
			formats = append(formats, format)
		}
	}

	if len(formats) == 0 {
		return "unknown"
	}

	slices.Sort(formats)

	return strings.Join(formats, " / ")
}

func HandleModelMetadata(w http.ResponseWriter, request *http.Request) {
	result := ModelMetadata{NativeQuantization: "unknown"}

	model := GetModel(request.URL.Query().Get("model"))

	if !env.IsOpenAI() && model != nil && model.HFSlug != "" {
		result.HFSlug = model.HFSlug

		metadata, err := huggingFaceCache.Get(request.Context(), model.HFSlug)
		if err != nil {
			log.Warnln(err)
		}

		result.NativeQuantization = metadata.NativeQuantization()
	}

	RespondJson(w, http.StatusOK, result)
}

func compressedTensorQuantization(config map[string]any) string {
	defaultFormat := compressedTensorFormat(config)
	groups, _ := config["config_groups"].(map[string]any)

	if len(groups) == 0 {
		return defaultFormat
	}

	formats := make([]string, 0, len(groups))

	for _, value := range groups {
		group, _ := value.(map[string]any)

		format := compressedTensorFormat(group)
		groupFormat, _ := group["format"].(string)

		if format == "" && strings.TrimSpace(groupFormat) == "" {
			format = defaultFormat
		}

		if format == "" {
			weights, _ := group["weights"].(map[string]any)

			bits, ok := weights["num_bits"]
			if !ok || bits == nil {
				return ""
			}

			kind, _ := weights["type"].(string)

			switch strings.ToLower(strings.TrimSpace(kind)) {
			case "float":
				format = fmt.Sprintf("FP%v", bits)
			case "int":
				format = fmt.Sprintf("INT%v", bits)
			default:
				format = fmt.Sprintf("%v-bit", bits)
			}
		}

		if !slices.Contains(formats, format) {
			formats = append(formats, format)
		}
	}

	slices.Sort(formats)

	return strings.Join(formats, " / ")
}

func compressedTensorFormat(config map[string]any) string {
	format, _ := config["format"].(string)

	switch strings.ToLower(strings.TrimSpace(format)) {
	case "mxfp4-pack-quantized":
		return "MXFP4"
	case "nvfp4-pack-quantized":
		return "NVFP4"
	default:
		return ""
	}
}
