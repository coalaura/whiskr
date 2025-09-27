package main

import (
	"bytes"
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
	ID      string  `msgpack:"id"`
	Name    string  `msgpack:"name"`
	Args    string  `msgpack:"args"`
	Result  string  `msgpack:"result,omitempty"`
	Done    bool    `msgpack:"done,omitempty"`
	Invalid bool    `msgpack:"invalid,omitempty"`
	Cost    float64 `msgpack:"cost,omitempty"`
}

type TextFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type Message struct {
	Role  string     `json:"role"`
	Text  string     `json:"text"`
	Tool  *ToolCall  `json:"tool"`
	Files []TextFile `json:"files"`
}

type Reasoning struct {
	Effort string `json:"effort"`
	Tokens int    `json:"tokens"`
}

type Tools struct {
	JSON   bool `json:"json"`
	Search bool `json:"search"`
}

type Metadata struct {
	Timezone string `json:"timezone"`
	Platform string `json:"platform"`
}

type Request struct {
	Prompt      string    `json:"prompt"`
	Model       string    `json:"model"`
	Temperature float64   `json:"temperature"`
	Iterations  int64     `json:"iterations"`
	Tools       Tools     `json:"tools"`
	Reasoning   Reasoning `json:"reasoning"`
	Metadata    Metadata  `json:"metadata"`
	Messages    []Message `json:"messages"`
}

func (t *ToolCall) AsAssistantToolCall(content string) openrouter.ChatCompletionMessage {
	// Some models require there to be content
	if content == "" {
		content = " "
	}

	return openrouter.ChatCompletionMessage{
		Role: openrouter.ChatMessageRoleAssistant,
		Content: openrouter.Content{
			Text: content,
		},
		ToolCalls: []openrouter.ToolCall{
			{
				ID:   t.ID,
				Type: openrouter.ToolTypeFunction,
				Function: openrouter.FunctionCall{
					Name:      t.Name,
					Arguments: t.Args,
				},
			},
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

func (r *Request) Parse() (*openrouter.ChatCompletionRequest, int, error) {
	var (
		request   openrouter.ChatCompletionRequest
		toolIndex int
	)

	model := GetModel(r.Model)
	if model == nil {
		return nil, 0, fmt.Errorf("unknown model: %q", r.Model)
	}

	request.Model = r.Model

	request.Modalities = []openrouter.ChatCompletionModality{
		openrouter.ModalityText,
	}

	if env.Settings.ImageGeneration && model.Images {
		request.Modalities = append(request.Modalities, openrouter.ModalityImage)
	}

	if r.Iterations < 1 || r.Iterations > 50 {
		return nil, 0, fmt.Errorf("invalid iterations (1-50): %d", r.Iterations)
	}

	if r.Temperature < 0 || r.Temperature > 2 {
		return nil, 0, fmt.Errorf("invalid temperature (0-2): %f", r.Temperature)
	}

	request.Temperature = float32(r.Temperature)

	if model.Reasoning {
		request.Reasoning = &openrouter.ChatCompletionReasoning{}

		switch r.Reasoning.Effort {
		case "high", "medium", "low":
			request.Reasoning.Effort = &r.Reasoning.Effort
		default:
			if r.Reasoning.Tokens <= 0 || r.Reasoning.Tokens > 1024*1024 {
				return nil, 0, fmt.Errorf("invalid reasoning tokens (1-1048576): %d", r.Reasoning.Tokens)
			}

			request.Reasoning.MaxTokens = &r.Reasoning.Tokens
		}
	}

	if model.JSON && r.Tools.JSON {
		request.ResponseFormat = &openrouter.ChatCompletionResponseFormat{
			Type: openrouter.ChatCompletionResponseFormatTypeJSONObject,
		}
	}

	prompt, err := BuildPrompt(r.Prompt, r.Metadata, model)
	if err != nil {
		return nil, 0, err
	}

	if prompt != "" {
		request.Messages = append(request.Messages, openrouter.SystemMessage(prompt))
	}

	if model.Tools && r.Tools.Search && env.Tokens.Exa != "" && r.Iterations > 1 {
		request.Tools = GetSearchTools()
		request.ToolChoice = "auto"

		toolIndex = len(request.Messages)

		request.Messages = append(
			request.Messages,
			openrouter.SystemMessage(""),
		)
	}

	for _, message := range r.Messages {
		message.Text = strings.ReplaceAll(message.Text, "\r", "")

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

			if len(message.Files) > 0 {
				if content.Text != "" {
					content.Multi = append(content.Multi, openrouter.ChatMessagePart{
						Type: openrouter.ChatMessagePartTypeText,
						Text: content.Text,
					})

					content.Text = ""
				}

				for i, file := range message.Files {
					if len(file.Name) > 512 {
						return nil, 0, fmt.Errorf("file %d is invalid (name too long, max 512 characters)", i)
					} else if len(file.Content) > 4*1024*1024 {
						return nil, 0, fmt.Errorf("file %d is invalid (too big, max 4MB)", i)
					}

					lines := strings.Count(file.Content, "\n") + 1

					content.Multi = append(content.Multi, openrouter.ChatMessagePart{
						Type: openrouter.ChatMessagePartTypeText,
						Text: fmt.Sprintf(
							"FILE %q LINES %d\n<<CONTENT>>\n%s\n<<END>>",
							file.Name,
							lines,
							file.Content,
						),
					})
				}
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
				msg = tool.AsAssistantToolCall(message.Text)

				request.Messages = append(request.Messages, msg)

				msg = tool.AsToolMessage()
			}

			request.Messages = append(request.Messages, msg)
		}
	}

	return &request, toolIndex, nil
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

	request, toolIndex, err := raw.Parse()
	if err != nil {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})

		return
	}

	request.Stream = true

	debug("preparing stream")

	ctx := r.Context()

	response, err := NewStream(w, ctx)
	if err != nil {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})

		return
	}

	debug("handling request")

	for iteration := range raw.Iterations {
		debug("iteration %d of %d", iteration+1, raw.Iterations)

		response.WriteChunk(NewChunk(ChunkStart, nil))

		if len(request.Tools) > 0 {
			if iteration == raw.Iterations-1 {
				debug("no more tool calls")

				request.Tools = nil
				request.ToolChoice = ""
			}

			// iterations - 1
			total := raw.Iterations - (iteration + 1)

			var tools bytes.Buffer

			InternalToolsTmpl.Execute(&tools, map[string]any{
				"total":     total,
				"remaining": total - 1,
			})

			request.Messages[toolIndex].Content.Text = tools.String()
		}

		dump("chat.json", request)

		tool, message, err := RunCompletion(ctx, response, request)
		if err != nil {
			response.WriteChunk(NewChunk(ChunkError, err))

			return
		}

		if tool == nil {
			debug("no tool call, done")

			return
		}

		debug("got %q tool call", tool.Name)

		response.WriteChunk(NewChunk(ChunkTool, tool))

		switch tool.Name {
		case "search_web":
			err = HandleSearchWebTool(ctx, tool)
			if err != nil {
				response.WriteChunk(NewChunk(ChunkError, err))

				return
			}
		case "fetch_contents":
			err = HandleFetchContentsTool(ctx, tool)
			if err != nil {
				response.WriteChunk(NewChunk(ChunkError, err))

				return
			}
		case "github_repository":
			err = HandleGitHubRepositoryTool(ctx, tool)
			if err != nil {
				response.WriteChunk(NewChunk(ChunkError, err))

				return
			}
		default:
			tool.Invalid = true
			tool.Result = "error: invalid tool call"
		}

		tool.Done = true

		debug("finished tool call")

		response.WriteChunk(NewChunk(ChunkTool, tool))

		request.Messages = append(request.Messages,
			tool.AsAssistantToolCall(message),
			tool.AsToolMessage(),
		)

		response.WriteChunk(NewChunk(ChunkEnd, nil))
	}
}

func RunCompletion(ctx context.Context, response *Stream, request *openrouter.ChatCompletionRequest) (*ToolCall, string, error) {
	stream, err := OpenRouterStartStream(ctx, *request)
	if err != nil {
		return nil, "", err
	}

	defer stream.Close()

	var (
		id    string
		open  int
		close int
		tool  *ToolCall
	)

	buf := GetFreeBuffer()
	defer pool.Put(buf)

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

			response.WriteChunk(NewChunk(ChunkID, id))
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]
		delta := choice.Delta

		if choice.FinishReason == openrouter.FinishReasonContentFilter {
			response.WriteChunk(NewChunk(ChunkError, errors.New("stopped due to content_filter")))

			return nil, "", nil
		}

		calls := delta.ToolCalls

		if len(calls) > 0 {
			call := calls[0]

			if open > 0 && open == close {
				continue
			}

			if tool == nil {
				tool = &ToolCall{}
			}

			if call.ID != "" && !strings.HasSuffix(tool.ID, call.ID) {
				tool.ID += call.ID
			}

			if call.Function.Name != "" && !strings.HasSuffix(tool.Name, call.Function.Name) {
				tool.Name += call.Function.Name
			}

			open += strings.Count(call.Function.Arguments, "{")
			close += strings.Count(call.Function.Arguments, "}")

			tool.Args += call.Function.Arguments
		} else if tool != nil {
			break
		}

		if delta.Content != "" {
			buf.WriteString(delta.Content)

			response.WriteChunk(NewChunk(ChunkText, delta.Content))
		} else if delta.Reasoning != nil {
			response.WriteChunk(NewChunk(ChunkReasoning, *delta.Reasoning))
		} else if len(delta.Images) > 0 {
			for _, image := range delta.Images {
				if image.Type != openrouter.StreamImageTypeImageURL {
					continue
				}

				response.WriteChunk(NewChunk(ChunkImage, image.ImageURL.URL))
			}
		}
	}

	return tool, buf.String(), nil
}
