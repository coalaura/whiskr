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
	"time"

	"github.com/revrost/go-openrouter"
)

type ToolReasoning struct {
	Format    string `msgpack:"format"`
	Encrypted string `msgpack:"encrypted"`
}

type ToolCall struct {
	ID        string         `msgpack:"id"`
	Name      string         `msgpack:"name"`
	Args      string         `msgpack:"args"`
	Result    string         `msgpack:"result,omitempty"`
	Done      bool           `msgpack:"done,omitempty"`
	Invalid   bool           `msgpack:"invalid,omitempty"`
	Cost      float64        `msgpack:"cost,omitempty"`
	Reasoning *ToolReasoning `msgpack:"reasoning,omitempty"`
}

type TextFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type Message struct {
	Role   string     `json:"role"`
	Text   string     `json:"text"`
	Tool   *ToolCall  `json:"tool"`
	Files  []TextFile `json:"files"`
	Images []string   `json:"images"`
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
	Timezone string   `json:"timezone"`
	Platform string   `json:"platform"`
	Settings Settings `json:"settings"`
}

type Settings struct {
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
}

type Request struct {
	Prompt      string    `json:"prompt"`
	Model       string    `json:"model"`
	Provider    string    `json:"provider"`
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

	call := openrouter.ChatCompletionMessage{
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

	if t.Reasoning != nil {
		call.ReasoningDetails = []openrouter.ChatCompletionReasoningDetails{
			{
				Type:   openrouter.ReasoningDetailsTypeEncrypted,
				Data:   t.Reasoning.Encrypted,
				ID:     t.ID,
				Format: t.Reasoning.Format,
				Index:  0,
			},
		}
	}

	return call
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

func (r *Request) AddToolPrompt(request *openrouter.ChatCompletionRequest, iteration int64) bool {
	if len(request.Tools) == 0 {
		return false
	}

	if iteration == r.Iterations-1 {
		debug("no more tool calls")

		request.Tools = nil
		request.ToolChoice = ""
	}

	// iterations - 1
	total := r.Iterations - (iteration + 1)

	var tools bytes.Buffer

	InternalToolsTmpl.Execute(&tools, map[string]any{
		"total":     total,
		"remaining": total - 1,
	})

	request.Messages = append(request.Messages, openrouter.SystemMessage(tools.String()))

	return true
}

func (r *Request) Parse() (*openrouter.ChatCompletionRequest, error) {
	var request openrouter.ChatCompletionRequest

	model := GetModel(r.Model)
	if model == nil {
		return nil, fmt.Errorf("unknown model: %q", r.Model)
	}

	request.Model = r.Model

	request.Modalities = []openrouter.ChatCompletionModality{
		openrouter.ModalityText,
	}

	if env.Settings.ImageGeneration && model.Images {
		request.Modalities = append(request.Modalities, openrouter.ModalityImage)
	}

	request.Transforms = append(request.Transforms, env.Settings.Transformation)

	if r.Iterations < 1 || r.Iterations > 50 {
		return nil, fmt.Errorf("invalid iterations (1-50): %d", r.Iterations)
	}

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

	switch r.Provider {
	case "throughput":
		request.Provider = &openrouter.ChatProvider{
			Sort: openrouter.ProviderSortingThroughput,
		}
	case "latency":
		request.Provider = &openrouter.ChatProvider{
			Sort: openrouter.ProviderSortingLatency,
		}
	case "price":
		request.Provider = &openrouter.ChatProvider{
			Sort: openrouter.ProviderSortingPrice,
		}
	}

	if model.JSON && r.Tools.JSON {
		request.ResponseFormat = &openrouter.ChatCompletionResponseFormat{
			Type: openrouter.ChatCompletionResponseFormatTypeJSONObject,
		}
	}

	prompt, err := BuildPrompt(r.Prompt, r.Metadata, model)
	if err != nil {
		return nil, err
	}

	if prompt != "" {
		request.Messages = append(request.Messages, openrouter.SystemMessage(prompt))
	}

	if model.Tools && r.Tools.Search && env.Tokens.Exa != "" && r.Iterations > 1 {
		request.Tools = GetSearchTools()
		request.ToolChoice = "auto"
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
			var (
				content openrouter.Content
				multi   bool
				last    = -1
			)

			if model.Vision && strings.Contains(message.Text, "![") {
				content.Multi = SplitImagePairs(message.Text)

				multi = true

				if content.Multi[len(content.Multi)-1].Type == openrouter.ChatMessagePartTypeText {
					last = len(content.Multi) - 1
				}
			} else {
				content.Text = message.Text
			}

			if len(message.Files) > 0 {
				for i, file := range message.Files {
					if len(file.Name) > 512 {
						return nil, fmt.Errorf("file %d is invalid (name too long, max 512 characters)", i)
					} else if len(file.Content) > 4*1024*1024 {
						return nil, fmt.Errorf("file %d is invalid (too big, max 4MB)", i)
					}

					lines := strings.Count(file.Content, "\n") + 1

					entry := fmt.Sprintf(
						"FILE %q LINES %d\n<<CONTENT>>\n%s\n<<END>>",
						file.Name,
						lines,
						file.Content,
					)

					if multi {
						if last != -1 {
							if content.Multi[last].Text != "" {
								content.Multi[last].Text += "\n\n"
							}

							content.Multi[last].Text += entry
						} else {
							content.Multi = append(content.Multi, openrouter.ChatMessagePart{
								Type: openrouter.ChatMessagePartTypeText,
								Text: entry,
							})
						}
					} else {
						if content.Text != "" {
							content.Text += "\n\n"
						}

						content.Text += entry
					}
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

			for index, image := range message.Images {
				msg.Images = append(msg.Images, openrouter.ChatCompletionImage{
					Index: index,
					Type:  openrouter.StreamImageTypeImageURL,
					ImageURL: openrouter.ChatCompletionImageURL{
						URL: image,
					},
				})
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

	return &request, nil
}

func ParseChatRequest(r *http.Request) (*Request, *openrouter.ChatCompletionRequest, error) {
	var raw Request

	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		return nil, nil, err
	}

	request, err := raw.Parse()
	if err != nil {
		return nil, nil, err
	}

	request.Stream = true

	return &raw, request, nil
}

func HandleDump(w http.ResponseWriter, r *http.Request) {
	debug("parsing dump")

	raw, request, err := ParseChatRequest(r)
	if err != nil {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})

		return
	}

	raw.AddToolPrompt(request, 0)

	RespondJson(w, http.StatusOK, map[string]any{
		"request": request,
	})
}

func HandleChat(w http.ResponseWriter, r *http.Request) {
	debug("parsing chat")

	raw, request, err := ParseChatRequest(r)
	if err != nil {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})

		return
	}

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

	go func() {
		ticker := time.NewTicker(5 * time.Second)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				response.WriteChunk(NewChunk(ChunkAlive, nil))
			}
		}
	}()

	for iteration := range raw.Iterations {
		debug("iteration %d of %d", iteration+1, raw.Iterations)

		response.WriteChunk(NewChunk(ChunkStart, nil))

		hasToolMessage := raw.AddToolPrompt(request, iteration)

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

		switch tool.Name {
		case "search_web":
			arguments, err := ParseAndUpdateArgs[SearchWebArguments](tool)
			if err != nil {
				response.WriteChunk(NewChunk(ChunkError, err))

				return
			}

			response.WriteChunk(NewChunk(ChunkTool, tool))

			err = HandleSearchWebTool(ctx, tool, arguments)
			if err != nil {
				response.WriteChunk(NewChunk(ChunkError, err))

				return
			}
		case "fetch_contents":
			arguments, err := ParseAndUpdateArgs[FetchContentsArguments](tool)
			if err != nil {
				response.WriteChunk(NewChunk(ChunkError, err))

				return
			}

			response.WriteChunk(NewChunk(ChunkTool, tool))

			err = HandleFetchContentsTool(ctx, tool, arguments)
			if err != nil {
				response.WriteChunk(NewChunk(ChunkError, err))

				return
			}
		case "github_repository":
			arguments, err := ParseAndUpdateArgs[GitHubRepositoryArguments](tool)
			if err != nil {
				response.WriteChunk(NewChunk(ChunkError, err))

				return
			}

			response.WriteChunk(NewChunk(ChunkTool, tool))

			err = HandleGitHubRepositoryTool(ctx, tool, arguments)
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

		if hasToolMessage {
			request.Messages = request.Messages[:len(request.Messages)-1]
		}

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
		return nil, "", fmt.Errorf("stream.start: %v", err)
	}

	defer stream.Close()

	var (
		id        string
		open      int
		close     int
		reasoning bool
		tool      *ToolCall
	)

	buf := GetFreeBuffer()
	defer pool.Put(buf)

	for {
		chunk, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, "", fmt.Errorf("stream.receive: %v", err)
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

			if len(delta.ReasoningDetails) != 0 && tool.Reasoning == nil {
				for _, details := range delta.ReasoningDetails {
					if details.Type != openrouter.ReasoningDetailsTypeEncrypted {
						continue
					}

					tool.Reasoning = &ToolReasoning{
						Format:    details.Format,
						Encrypted: details.Data,
					}
				}
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
			if !reasoning && len(delta.ReasoningDetails) != 0 {
				reasoning = true

				response.WriteChunk(NewChunk(ChunkReasoningType, delta.ReasoningDetails[0].Type))
			}

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
