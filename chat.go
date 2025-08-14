package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/revrost/go-openrouter"
)

type ToolCall struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Args   string `json:"args"`
	Result string `json:"result,omitempty"`
}

type Message struct {
	Role string    `json:"role"`
	Text string    `json:"text"`
	Tool *ToolCall `json:"tool"`
}

type Reasoning struct {
	Effort string `json:"effort"`
	Tokens int    `json:"tokens"`
}

type Request struct {
	Prompt      string    `json:"prompt"`
	Model       string    `json:"model"`
	Temperature float64   `json:"temperature"`
	JSON        bool      `json:"json"`
	Search      bool      `json:"search"`
	Reasoning   Reasoning `json:"reasoning"`
	Messages    []Message `json:"messages"`
}

func (t *ToolCall) AsToolCall() openrouter.ToolCall {
	return openrouter.ToolCall{
		ID:   t.ID,
		Type: openrouter.ToolTypeFunction,
		Function: openrouter.FunctionCall{
			Name:      t.Name,
			Arguments: t.Args,
		},
	}
}

func (t *ToolCall) AsToolMessage() openrouter.ChatCompletionMessage {
	return openrouter.ChatCompletionMessage{
		Role:       openrouter.ChatMessageRoleTool,
		ToolCallID: t.ID,
		Content: openrouter.Content{
			Text: t.Result,
		},
	}
}

func (r *Request) Parse() (*openrouter.ChatCompletionRequest, error) {
	var request openrouter.ChatCompletionRequest

	model, ok := ModelMap[r.Model]
	if !ok {
		return nil, fmt.Errorf("unknown model: %q", r.Model)
	}

	request.Model = r.Model

	if r.Temperature < 0 || r.Temperature > 2 {
		return nil, fmt.Errorf("invalid temperature (0-2): %f", r.Temperature)
	}

	request.Temperature = float32(r.Temperature)

	if model.Reasoning {
		request.Reasoning = &openrouter.ChatCompletionReasoning{}

		switch r.Reasoning.Effort {
		case "high", "medium", "low":
			request.Reasoning.Effort = &r.Reasoning.Effort
		default:
			if r.Reasoning.Tokens <= 0 || r.Reasoning.Tokens > 1024*1024 {
				return nil, fmt.Errorf("invalid reasoning tokens (1-1048576): %d", r.Reasoning.Tokens)
			}

			request.Reasoning.MaxTokens = &r.Reasoning.Tokens
		}
	}

	if model.JSON && r.JSON {
		request.ResponseFormat = &openrouter.ChatCompletionResponseFormat{
			Type: openrouter.ChatCompletionResponseFormatTypeJSONObject,
		}
	}

	if model.Tools && r.Search {
		request.Tools = GetSearchTool()
		request.ToolChoice = "auto"
	}

	prompt, err := BuildPrompt(r.Prompt, model)
	if err != nil {
		return nil, err
	}

	if prompt != "" {
		request.Messages = append(request.Messages, openrouter.SystemMessage(prompt))
	}

	for index, message := range r.Messages {
		switch message.Role {
		case "system", "user":
			request.Messages = append(request.Messages, openrouter.ChatCompletionMessage{
				Role: message.Role,
				Content: openrouter.Content{
					Text: message.Text,
				},
			})
		case "assistant":
			msg := openrouter.ChatCompletionMessage{
				Role: openrouter.ChatMessageRoleAssistant,
				Content: openrouter.Content{
					Text: message.Text,
				},
			}

			tool := message.Tool
			if tool != nil {
				msg.ToolCalls = []openrouter.ToolCall{tool.AsToolCall()}

				request.Messages = append(request.Messages, msg)

				msg = tool.AsToolMessage()
			}

			request.Messages = append(request.Messages, msg)
		default:
			return nil, fmt.Errorf("[%d] invalid role: %q", index+1, message.Role)
		}
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
	dump(request)

	response, err := NewStream(w)
	if err != nil {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})

		return
	}

	ctx := r.Context()

	for iteration := range MaxIterations {
		if iteration == MaxIterations-1 {
			request.Tools = nil
			request.ToolChoice = ""
		}

		tool, message, err := RunCompletion(ctx, response, request)
		if err != nil {
			response.Send(ErrorChunk(err))

			return
		}

		if tool == nil || tool.Name != "search_internet" {
			return
		}

		response.Send(ToolChunk(tool))

		err = HandleSearchTool(ctx, tool)
		if err != nil {
			response.Send(ErrorChunk(err))

			return
		}

		response.Send(ToolChunk(tool))

		request.Messages = append(request.Messages,
			openrouter.ChatCompletionMessage{
				Role: openrouter.ChatMessageRoleAssistant,
				Content: openrouter.Content{
					Text: message,
				},
				ToolCalls: []openrouter.ToolCall{tool.AsToolCall()},
			},
			tool.AsToolMessage(),
		)
	}
}

func RunCompletion(ctx context.Context, response *Stream, request *openrouter.ChatCompletionRequest) (*ToolCall, string, error) {
	stream, err := OpenRouterStartStream(ctx, *request)
	if err != nil {
		return nil, "", err
	}

	defer stream.Close()

	var (
		id     string
		result strings.Builder
		tool   *ToolCall
	)

	for {
		chunk, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			log.Warning("stream error")
			log.WarningE(err)

			return nil, "", err
		}

		if id == "" {
			id = chunk.ID

			response.Send(IDChunk(id))
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]

		// DEBUG
		debug(choice)

		if choice.FinishReason == openrouter.FinishReasonContentFilter {
			response.Send(ErrorChunk(errors.New("stopped due to content_filter")))

			return nil, "", nil
		}

		calls := choice.Delta.ToolCalls

		if len(calls) > 0 {
			call := calls[0]

			if tool == nil {
				tool = &ToolCall{}
			}

			tool.ID += call.ID
			tool.Name += call.Function.Name
			tool.Args += call.Function.Arguments
		} else if tool != nil {
			break
		}

		content := choice.Delta.Content

		if content != "" {
			result.WriteString(content)

			response.Send(TextChunk(content))
		} else if choice.Delta.Reasoning != nil {
			response.Send(ReasoningChunk(*choice.Delta.Reasoning))
		}
	}

	return tool, result.String(), nil
}
