package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/revrost/go-openrouter"
)

type Message struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

type Request struct {
	Prompt      string    `json:"prompt"`
	Model       string    `json:"model"`
	Temperature float64   `json:"temperature"`
	Messages    []Message `json:"messages"`
}

func (r *Request) Parse() (*openrouter.ChatCompletionRequest, error) {
	var request openrouter.ChatCompletionRequest

	model, ok := ModelMap[r.Model]
	if !ok {
		return nil, fmt.Errorf("unknown model: %q", r.Model)
	}

	request.Model = r.Model

	if r.Temperature < 0 || r.Temperature > 1 {
		return nil, fmt.Errorf("invalid temperature (0-1): %f", r.Temperature)
	}

	request.Temperature = float32(r.Temperature)

	prompt, err := BuildPrompt(r.Prompt, model)
	if err != nil {
		return nil, err
	}

	if prompt != "" {
		request.Messages = append(request.Messages, openrouter.SystemMessage(prompt))
	}

	for index, message := range r.Messages {
		if message.Role != openrouter.ChatMessageRoleSystem && message.Role != openrouter.ChatMessageRoleAssistant && message.Role != openrouter.ChatMessageRoleUser {
			return nil, fmt.Errorf("[%d] invalid role: %q", index+1, message.Role)
		}

		request.Messages = append(request.Messages, openrouter.ChatCompletionMessage{
			Role: message.Role,
			Content: openrouter.Content{
				Text: message.Text,
			},
		})
	}

	h := "high"

	request.Reasoning = &openrouter.ChatCompletionReasoning{
		Effort: &h,
	}

	return &request, nil
}

func HandleChat(w http.ResponseWriter, r *http.Request) {
	var raw Request

	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})

		return
	}

	request, err := raw.Parse()
	if err != nil {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})

		return
	}

	request.Stream = true

	// DEBUG
	b, _ := json.MarshalIndent(request, "", "\t")
	os.WriteFile("debug.json", b, 0755)

	ctx := r.Context()

	stream, err := OpenRouterStartStream(ctx, *request)
	if err != nil {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})

		return
	}

	defer stream.Close()

	response, err := NewStream(w)
	if err != nil {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})

		return
	}

	for {
		chunk, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}

			response.Send(ErrorChunk(err))

			return
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]

		if choice.FinishReason == openrouter.FinishReasonContentFilter {
			response.Send(ErrorChunk(errors.New("stopped due to content_filter")))

			return
		}

		content := choice.Delta.Content

		if content != "" {
			response.Send(TextChunk(content))
		} else if choice.Delta.Reasoning != nil {
			response.Send(ReasoningChunk(*choice.Delta.Reasoning))
		}
	}
}
