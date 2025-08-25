package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/revrost/go-openrouter"
	"github.com/revrost/go-openrouter/jsonschema"
)

type TitleRequest struct {
	Title    *string   `json:"title"`
	Messages []Message `json:"messages"`
}

type TitleResponse struct {
	Title string  `json:"title"`
	Cost  float64 `json:"cost,omitempty"`
}

var (
	titleReplacer = strings.NewReplacer(
		"\r", "",
		"\n", "\\n",
		"\t", "\\t",
	)

	titleSchema, _ = jsonschema.GenerateSchema[TitleResponse]()
)

func HandleTitle(w http.ResponseWriter, r *http.Request) {
	debug("parsing title")

	var raw TitleRequest

	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})

		return
	}

	debug("preparing request")

	messages := make([]string, 0, len(raw.Messages))

	for _, message := range raw.Messages {
		switch message.Role {
		case "system", "assistant", "user":
			text := message.Text

			if len(message.Files) != 0 {
				if text != "" {
					text += "\n"
				}

				files := make([]string, len(message.Files))

				for i, file := range message.Files {
					files[i] = file.Name
				}

				text += fmt.Sprintf("FILES: %s", strings.Join(files, ", "))
			}

			if text != "" {
				text = strings.TrimSpace(text)
				text = titleReplacer.Replace(text)

				messages = append(messages, fmt.Sprintf("%s: %s", strings.ToUpper(message.Role), text))
			}
		}
	}

	if len(messages) == 0 {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": "no valid messages",
		})

		return
	}

	var prompt strings.Builder

	if err := InternalTitleTmpl.Execute(&prompt, raw); err != nil {
		RespondJson(w, http.StatusInternalServerError, map[string]any{
			"error": err.Error(),
		})

		return
	}

	request := openrouter.ChatCompletionRequest{
		Model: env.Settings.TitleModel,
		Messages: []openrouter.ChatCompletionMessage{
			openrouter.SystemMessage(prompt.String()),
			openrouter.UserMessage(strings.Join(messages, "\n")),
		},
		Temperature: 0.25,
		MaxTokens:   100,
		ResponseFormat: &openrouter.ChatCompletionResponseFormat{
			Type: openrouter.ChatCompletionResponseFormatTypeJSONSchema,
			JSONSchema: &openrouter.ChatCompletionResponseFormatJSONSchema{
				Name:   "chat_title",
				Schema: titleSchema,
				Strict: true,
			},
		},
		Usage: &openrouter.IncludeUsage{
			Include: true,
		},
	}

	if raw.Title != nil {
		request.Temperature = 0.4
	}

	dump("title.json", request)

	debug("generating title")

	response, err := OpenRouterRun(r.Context(), request)
	if err != nil {
		RespondJson(w, http.StatusInternalServerError, map[string]any{
			"error": err.Error(),
		})

		return
	}

	choice := response.Choices[0].Message.Content.Text
	cost := response.Usage.Cost

	var result TitleResponse

	err = json.Unmarshal([]byte(choice), &result)
	if err != nil {
		RespondJson(w, http.StatusInternalServerError, map[string]any{
			"error": err.Error(),
			"cost":  cost,
		})

		return
	}

	result.Cost = cost

	RespondJson(w, http.StatusOK, result)
}
