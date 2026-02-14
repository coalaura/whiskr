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
	Filename *string   `json:"filename"`
	Messages []Message `json:"messages"`
}

type TitleResponse struct {
	Title    string `json:"title"`
	Filename string `json:"filename"`
}

const (
	TitleHeadCount = 3
	TitleTailCount = 3
)

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

	selected := selectTitleMessages(raw.Messages, raw.Title != nil)

	messages := make([]string, 0, len(selected))

	for _, message := range selected {
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

	buf := GetFreeBuffer()
	defer pool.Put(buf)

	if err := InternalTitleTmpl.Execute(buf, raw); err != nil {
		RespondJson(w, http.StatusInternalServerError, map[string]any{
			"error": err.Error(),
		})

		return
	}

	request := openrouter.ChatCompletionRequest{
		Model: env.Models.TitleModel,
		Messages: []openrouter.ChatCompletionMessage{
			openrouter.SystemMessage(buf.String()),
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
	cost := response.Usage.Cost + response.Usage.CostDetails.UpstreamInferenceCost

	var result TitleResponse

	err = json.Unmarshal([]byte(choice), &result)
	if err != nil {
		RespondJson(w, http.StatusInternalServerError, map[string]any{
			"error": err.Error(),
			"cost":  cost,
		})

		return
	}

	RespondJson(w, http.StatusOK, map[string]any{
		"title": result.Title,
		"file":  result.Filename,
		"cost":  cost,
	})
}

func selectTitleMessages(msgs []Message, retitle bool) []Message {
	total := len(msgs)

	if total == 0 {
		return msgs
	}

	// small conversations: send everything (truncated)
	if total <= TitleHeadCount+TitleTailCount {
		out := make([]Message, len(msgs))
		copy(out, msgs)

		for i := range out {
			out[i].Text = truncateText(out[i].Text, 512)
		}

		return out
	}

	result := make([]Message, 0, TitleHeadCount+TitleTailCount+1)

	// Head
	for i := 0; i < TitleHeadCount; i++ {
		msg := msgs[i]
		msg.Text = truncateText(msg.Text, 512)

		result = append(result, msg)
	}

	// summarize middle section
	middleStart := TitleHeadCount
	middleEnd := total - TitleTailCount

	var topics []string

	for i := middleStart; i < middleEnd; i++ {
		if msgs[i].Role != "user" {
			continue
		}

		text := strings.TrimSpace(msgs[i].Text)

		if text == "" {
			continue
		}

		summary := truncateText(text, 150)

		topics = append(topics, fmt.Sprintf("- %s", summary))
	}

	if len(topics) > 0 {
		if len(topics) > 12 {
			kept := make([]string, 0, 12)

			step := float64(len(topics)) / 12.0

			for i := 0; i < 12; i++ {
				kept = append(kept, topics[int(float64(i)*step)])
			}

			topics = kept
		}

		result = append(result, Message{
			Role: "system",
			Text: fmt.Sprintf(
				"[%d messages omitted from middle of conversation. User topics discussed:]\n%s",
				middleEnd-middleStart,
				strings.Join(topics, "\n"),
			),
		})
	} else {
		result = append(result, Message{
			Role: "system",
			Text: fmt.Sprintf("[%d messages omitted from middle of conversation]", middleEnd-middleStart),
		})
	}

	// tail
	for i := middleEnd; i < total; i++ {
		msg := msgs[i]
		msg.Text = truncateText(msg.Text, 512)

		result = append(result, msg)
	}

	return result
}

func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}

	return text[:maxLen] + "..."
}
