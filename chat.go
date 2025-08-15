package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/revrost/go-openrouter"
)

type ToolCall struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Args   string `json:"args"`
	Result string `json:"result,omitempty"`
	Done   bool   `json:"done,omitempty"`
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

	if model.Tools && r.Search && ExaToken != "" {
		request.Tools = GetSearchTools()
		request.ToolChoice = "auto"

		request.Messages = append(request.Messages, openrouter.SystemMessage("You have access to web search tools. Use `search_web` with `query` (string) and `num_results` (1-10) to find current information and get result summaries. Use `fetch_contents` with `urls` (array) to read full page content. Always specify all parameters for each tool call. Call only one tool per response."))
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
		case "system":
			request.Messages = append(request.Messages, openrouter.ChatCompletionMessage{
				Role: message.Role,
				Content: openrouter.Content{
					Text: message.Text,
				},
			})
		case "user":
			var content openrouter.Content

			if model.Vision && strings.Contains(message.Text, "![") {
				content.Multi = SplitImagePairs(message.Text)
			} else {
				content.Text = message.Text
			}

			request.Messages = append(request.Messages, openrouter.ChatCompletionMessage{
				Role:    message.Role,
				Content: content,
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
	debug("parsing chat")

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

	dump("debug.json", request)
	debug("preparing stream")

	response, err := NewStream(w)
	if err != nil {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})

		return
	}

	debug("handling request")

	ctx := r.Context()

	for iteration := range MaxIterations {
		debug("iteration %d of %d", iteration+1, MaxIterations)

		if iteration == MaxIterations-1 {
			debug("no more tool calls")

			request.Tools = nil
			request.ToolChoice = ""

			request.Messages = append(request.Messages, openrouter.SystemMessage("You have reached the maximum number of tool calls for this conversation. Provide your final response based on the information you have gathered."))
		}

		tool, message, err := RunCompletion(ctx, response, request)
		if err != nil {
			response.Send(ErrorChunk(err))

			return
		}

		if tool == nil {
			debug("no tool call, done")

			return
		}

		debug("got %q tool call", tool.Name)

		response.Send(ToolChunk(tool))

		switch tool.Name {
		case "search_web":
			err = HandleSearchWebTool(ctx, tool)
			if err != nil {
				response.Send(ErrorChunk(err))

				return
			}
		case "fetch_contents":
			err = HandleFetchContentsTool(ctx, tool)
			if err != nil {
				response.Send(ErrorChunk(err))

				return
			}
		default:
			return
		}

		tool.Done = true

		debug("finished tool call")

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

func SplitImagePairs(text string) []openrouter.ChatMessagePart {
	rgx := regexp.MustCompile(`(?m)!\[[^\]]*]\((\S+?)\)`)

	var (
		index int
		parts []openrouter.ChatMessagePart
	)

	push := func(str, end int) {
		rest := text[str:end]

		if rest == "" {
			return
		}

		total := len(parts)

		if total > 0 && parts[total-1].Type == openrouter.ChatMessagePartTypeText {
			parts[total-1].Text += rest

			return
		}

		parts = append(parts, openrouter.ChatMessagePart{
			Type: openrouter.ChatMessagePartTypeText,
			Text: rest,
		})
	}

	for {
		location := rgx.FindStringSubmatchIndex(text[index:])
		if location == nil {
			push(index, len(text)-1)

			break
		}

		start := index + location[0]
		end := index + location[1]

		urlStart := index + location[2]
		urlEnd := index + location[3]

		url := text[urlStart:urlEnd]

		if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
			push(index, end)

			index = end

			continue
		}

		if start > index {
			push(index, start)
		}

		parts = append(parts, openrouter.ChatMessagePart{
			Type: openrouter.ChatMessagePartTypeImageURL,
			ImageURL: &openrouter.ChatMessageImageURL{
				Detail: openrouter.ImageURLDetailAuto,
				URL:    url,
			},
		})

		index = end
	}

	return parts
}
