package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/coalaura/openingrouter"
)

type ChatToolReasoning struct {
	Format    string `msgpack:"format"`
	Encrypted string `msgpack:"encrypted"`
}

type ChatToolCall struct {
	ID        string             `msgpack:"id"`
	Name      string             `msgpack:"name"`
	Args      string             `msgpack:"args"`
	Result    string             `msgpack:"result,omitempty"`
	Done      bool               `msgpack:"done,omitempty"`
	Invalid   bool               `msgpack:"invalid,omitempty"`
	Cost      float64            `msgpack:"cost,omitempty"`
	Reasoning *ChatToolReasoning `msgpack:"reasoning,omitempty"`
}

type ChatTextFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type ChatMessage struct {
	Role   string         `json:"role"`
	Text   string         `json:"text"`
	Tool   *ChatToolCall  `json:"tool"`
	Files  []ChatTextFile `json:"files"`
	Images []string       `json:"images"`
}

type ChatImage struct {
	Resolution string `json:"resolution"`
	Aspect     string `json:"aspect"`
	MaxImages  int    `json:"max_images"`
}

type ChatTools struct {
	Images  bool `json:"images"`
	Files   bool `json:"files"`
	JSON    bool `json:"json"`
	Search  bool `json:"search"`
	Bare    bool `json:"bare"`
	Offline bool `json:"offline"`
}

type ChatMetadata struct {
	Timezone string       `json:"timezone"`
	Platform string       `json:"platform"`
	Settings ChatSettings `json:"settings"`
	Time     *int64       `json:"time"`
}

type ChatSettings struct {
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
}

// gost:preserve-layout
type ChatRequest struct {
	proxy *EnvProxy

	ProxyName   string        `json:"proxy"`
	Prompt      string        `json:"prompt"`
	Model       string        `json:"model"`
	Provider    string        `json:"provider"`
	ProviderPin string        `json:"provider_pin"`
	Temperature float64       `json:"temperature"`
	Iterations  int64         `json:"iterations"`
	Tools       ChatTools     `json:"tools"`
	Image       ChatImage     `json:"image"`
	Reasoning   string        `json:"reasoning"`
	Compression bool          `json:"compression"`
	Metadata    ChatMetadata  `json:"metadata"`
	Messages    []ChatMessage `json:"messages"`
}

var (
	nativeFinishReasons = map[string]string{
		// Google / Gemini Models
		"STOP": "",

		"FINISH_REASON_UNSPECIFIED": "unknown reason",
		"MAX_TOKENS":                "token limit reached",
		"OTHER":                     "unknown reason",
		"SAFETY":                    "safety filter",
		"BLOCKLIST":                 "blocklist trigger",
		"PROHIBITED_CONTENT":        "prohibited content",
		"SPII":                      "sensitive info (PII) filter",
		"RECITATION":                "copyright/recitation filter",
		"MODEL_ARMOR":               "security filter (Model Armor)",
		"IMAGE_SAFETY":              "image safety filter",
		"IMAGE_PROHIBITED_CONTENT":  "prohibited image content",
		"IMAGE_RECITATION":          "image recitation filter",
		"IMAGE_OTHER":               "unknown image error",
		"NO_IMAGE":                  "failed to generate image",
		"MALFORMED_FUNCTION_CALL":   "invalid function call",
		"UNEXPECTED_TOOL_CALL":      "unexpected tool call",
	}
)

func (t *ChatToolCall) AsAssistantToolCall(content string) openingrouter.ChatMessage {
	// Some models require there to be content
	if content == "" {
		content = " "
	}

	call := openingrouter.ChatMessage{
		Role: openingrouter.ChatRoleAssistant,
		Content: openingrouter.ChatContent{
			Text: content,
		},
		ToolCalls: []openingrouter.ChatToolCall{
			{
				ID:   t.ID,
				Type: openingrouter.ChatToolTypeFunction,
				Function: openingrouter.ChatToolCallFunction{
					Name:      t.Name,
					Arguments: t.Args,
				},
			},
		},
	}

	if t.Reasoning != nil {
		call.ReasoningDetails = []openingrouter.ChatReasoningDetail{
			{
				Type:   openingrouter.ChatReasoningDetailTypeEncrypted,
				Data:   t.Reasoning.Encrypted,
				ID:     t.ID,
				Format: openingrouter.ChatReasoningFormat(t.Reasoning.Format),
				Index:  0,
			},
		}
	}

	return call
}

func (t *ChatToolCall) AsToolMessage() openingrouter.ChatMessage {
	return openingrouter.ChatMessage{
		Role:       openingrouter.ChatRoleTool,
		ToolCallID: t.ID,
		Content: openingrouter.ChatContent{
			Text: t.Result,
		},
	}
}

func hasToolCallHistory(messages []openingrouter.ChatMessage) bool {
	for _, msg := range messages {
		if len(msg.ToolCalls) > 0 {
			return true
		}
	}

	return false
}

func (r *ChatRequest) AddToolPrompt(request *openingrouter.ChatCompletionRequest, iteration int64) bool {
	if r.Tools.Bare {
		return false
	}

	hasHistory := hasToolCallHistory(request.Messages)
	needExplicitStop := hasHistory && r.Prompt != ""

	if len(request.Tools) == 0 {
		if needExplicitStop {
			request.Messages = append(request.Messages, openingrouter.SystemMessage("Do not perform any more search tool calls."))
		}

		return false
	}

	isLastIteration := iteration == r.Iterations-1

	if isLastIteration {
		debug("no more tool calls")

		request.Tools = nil
		request.ToolChoice = nil
	}

	// iterations - 1
	total := r.Iterations - (iteration + 1)

	var tools bytes.Buffer

	InternalToolsTmpl.Execute(&tools, map[string]any{
		"total":     total,
		"remaining": total - 1,
	})

	request.Messages = append(request.Messages, openingrouter.SystemMessage(tools.String()))

	if isLastIteration && needExplicitStop {
		request.Messages = append(request.Messages, openingrouter.SystemMessage("Do not perform any more search tool calls."))
	}

	return true
}

func (r *ChatRequest) Parse() (*openingrouter.ChatCompletionRequest, error) {
	var request openingrouter.ChatCompletionRequest

	proxy, err := ResolveProxy(r.ProxyName)
	if err != nil {
		return nil, err
	}

	r.proxy = proxy

	model := GetModel(r.Model)
	if model == nil {
		return nil, fmt.Errorf("unknown model: %q", r.Model)
	}

	request.Model = r.Model

	request.MetadataLevel = openingrouter.ChatMetadataLevelEnabled

	if !model.IsTextOnly && model.Text {
		request.Modalities = append(request.Modalities, openingrouter.OutputModalityText)
	}

	if env.Models.ImageGeneration && model.Images {
		request.Modalities = append(request.Modalities, openingrouter.OutputModalityImage)

		imageConfig := openingrouter.ChatImageConfig{
			"image_size": "1K",
		}

		switch r.Image.Resolution {
		case "2K":
			imageConfig["image_size"] = "2K"
		case "4K":
			imageConfig["image_size"] = "4K"
		}

		switch r.Image.Aspect {
		case "1:1", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16", "16:9", "21:9":
			imageConfig["aspect_ratio"] = r.Image.Aspect
		}

		request.ImageConfig = imageConfig
	}

	if env.Models.Transformation != "" {
		engine := openingrouter.ChatContextCompressionEngine(env.Models.Transformation)
		request.Plugins = append(request.Plugins, openingrouter.ChatContextCompressionPlugin{
			ID:     openingrouter.ChatPluginIDContextCompression,
			Engine: engine,
		})
	}

	if r.Iterations < 1 || r.Iterations > 50 {
		return nil, fmt.Errorf("invalid iterations (1-50): %d", r.Iterations)
	}

	if r.Temperature < 0 || r.Temperature > 2 {
		return nil, fmt.Errorf("invalid temperature (0-2): %f", r.Temperature)
	}

	temperature := r.Temperature
	request.Temperature = &temperature

	if model.Reasoning {
		request.Reasoning = &openingrouter.ChatReasoningConfig{}

		switch r.Reasoning {
		case "xhigh", "high", "medium", "low", "minimal", "none":
			request.Reasoning.Effort = openingrouter.ReasoningEffort(r.Reasoning)
		}

		if len(model.ReasoningLevels) > 0 && !slices.Contains(model.ReasoningLevels, r.Reasoning) {
			return nil, fmt.Errorf("%q does not support effort %q", model.Name, r.Reasoning)
		}
	}

	prefs := &openingrouter.ProviderPreferences{}

	switch r.Provider {
	case "throughput":
		prefs.Sort = &openingrouter.ProviderSortConfig{By: openingrouter.ProviderSortThroughput}
	case "latency":
		prefs.Sort = &openingrouter.ProviderSortConfig{By: openingrouter.ProviderSortLatency}
	case "price":
		prefs.Sort = &openingrouter.ProviderSortConfig{By: openingrouter.ProviderSortPrice}
	}

	if r.ProviderPin != "" {
		prefs.Only = []string{r.ProviderPin}
	}

	if prefs.Sort != nil || len(prefs.Only) > 0 {
		request.Provider = prefs
	}

	if model.JSON && r.Tools.JSON {
		request.ResponseFormat = &openingrouter.ChatResponseFormat{
			Type: openingrouter.ChatResponseFormatTypeJSONObject,
		}
	}

	prompt, err := BuildPrompt(r.Prompt, r.Metadata, model, r.Tools.Bare)
	if err != nil {
		return nil, err
	}

	if !r.Tools.Bare {
		if r.Tools.Files {
			if prompt != "" {
				prompt += "\n\n"
			}

			prompt += InternalFilesPrompt
		} else {
			var hasFiles bool

			for _, message := range r.Messages {
				if message.Role == "user" && len(message.Files) > 0 {
					hasFiles = true

					break
				}
			}

			if hasFiles {
				if prompt != "" {
					prompt += "\n\n"
				}

				prompt += InternalNoFilesPrompt
			}
		}
	}

	if prompt != "" && !r.Tools.Bare {
		// volatile context after the cacheable system-prompt prefix.
		prompt += "\n\nCurrent date and time: " + FormatPromptDate(r.Metadata)
	}

	if prompt != "" {
		request.Messages = append(request.Messages, openingrouter.SystemMessage(prompt))
	}

	if model.Tools && r.Tools.Search && env.Tokens.Tavily != "" {
		if r.Iterations > 1 {
			request.Tools = GetSearchTools()
			request.ToolChoice = &openingrouter.ChatToolChoice{
				Mode: openingrouter.ChatToolChoiceModeAuto,
			}
		}
	} else {
		r.Iterations = 1
	}

	lastUser := -1

	for i, message := range r.Messages {
		if message.Role == "user" {
			lastUser = i
		}
	}

	for i, message := range r.Messages {
		message.Text = strings.ReplaceAll(message.Text, "\r", "")

		switch message.Role {
		case "system":
			request.Messages = append(request.Messages, openingrouter.ChatMessage{
				Role: openingrouter.ChatRoleSystem,
				Content: openingrouter.ChatContent{
					Text: message.Text,
				},
			})
		case "user":
			var (
				content openingrouter.ChatContent
				multi   bool
				last    = -1
			)

			if strings.Contains(message.Text, "![") {
				content.Parts = SplitImagePairs(message.Text, !model.Vision)

				multi = true

				if content.Parts[len(content.Parts)-1].Type == openingrouter.ChatContentPartTypeText {
					last = len(content.Parts) - 1
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

					clean := strings.ReplaceAll(file.Content, "</file>", "<\\/file>")

					entry := fmt.Sprintf(
						"<file name=%q>\n%s\n</file>",
						file.Name,
						clean,
					)

					if multi {
						if last != -1 {
							if content.Parts[last].Text != "" {
								content.Parts[last].Text += "\n\n"
							}

							content.Parts[last].Text += entry
						} else {
							content.Parts = append(content.Parts, openingrouter.ChatContentPart{
								Type: openingrouter.ChatContentPartTypeText,
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

			request.Messages = append(request.Messages, openingrouter.ChatMessage{
				Role:    openingrouter.ChatRoleUser,
				Content: content,
			})
		case "assistant":
			msg := openingrouter.ChatMessage{
				Role: openingrouter.ChatRoleAssistant,
				Content: openingrouter.ChatContent{
					Text: message.Text,
				},
			}

			for _, image := range message.Images {
				msg.Images = append(msg.Images, openingrouter.ChatAssistantImage{
					ImageURL: openingrouter.ContentPartImageURL{
						URL: image,
					},
				})
			}

			tool := message.Tool
			if tool != nil {
				msg = tool.AsAssistantToolCall(message.Text)

				request.Messages = append(request.Messages, msg)

				msg = tool.AsToolMessage()

				if r.Compression && i < lastUser {
					msg.Content = openingrouter.ChatContent{
						Text: "(result omitted)",
					}
				}
			}

			request.Messages = append(request.Messages, msg)
		}
	}

	maxImages := r.Image.MaxImages

	if maxImages < 0 {
		return nil, fmt.Errorf("invalid maximum images: %d", maxImages)
	}

	LimitChatRequestImages(&request, maxImages)

	return &request, nil
}

func ParseChatRequest(r *http.Request) (*ChatRequest, *openingrouter.ChatCompletionRequest, error) {
	var raw ChatRequest

	err := json.NewDecoder(r.Body).Decode(&raw)
	if err != nil {
		return nil, nil, err
	}

	request, err := raw.Parse()
	if err != nil {
		return nil, nil, err
	}

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
		defer ticker.Stop()

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

		response.WriteChunk(NewChunk(ChunkStart, StartChunk{
			Iteration: iteration + 1,
			Total:     raw.Iterations,
		}))

		hasToolMessage := raw.AddToolPrompt(request, iteration)

		dump("chat.json", request)

		tool, message, err := RunCompletion(ctx, response, request, raw.proxy)
		if err != nil {
			response.WriteChunk(NewChunk(ChunkError, err))

			return
		}

		if tool == nil {
			debug("no tool call, done")

			return
		}

		debug("got %q tool call", tool.Name)

		if len(request.Tools) == 0 {
			response.WriteChunk(NewChunk(ChunkError, fmt.Errorf("got %q tool call", tool.Name)))

			continue
		}

		if raw.Tools.Offline {
			tool.Result = "error: tool unavailable: network is offline"
		} else {

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

func RunCompletion(ctx context.Context, response *Stream, request *openingrouter.ChatCompletionRequest, proxy *EnvProxy) (*ChatToolCall, string, error) {
	started := time.Now()

	var (
		id             string
		open           int
		close          int
		completing     bool
		reasoning      bool
		hasContent     bool
		tool           *ChatToolCall
		statistics     *Statistics
		finish         openingrouter.ChatFinishReason
		native         string
		ttftMs         int64
		ttfoMs         int64
		reasoningStart int64
		outputImages   int
	)

	markToken := func(output bool) {
		elapsed := time.Since(started).Milliseconds()

		if ttftMs == 0 {
			ttftMs = elapsed
		}

		if output && ttfoMs == 0 {
			ttfoMs = elapsed
		}
	}

	stream, err := OpenRouterStartStream(ctx, *request, proxy)
	if err != nil {
		return nil, "", err
	}

	defer stream.Close()

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

		if chunk.Usage != nil {
			provider := streamProvider(chunk.OpenRouterMetadata)

			debug("usage chunk: model=%q provider=%q prompt=%d completion=%d cost=%v", chunk.Model, provider, chunk.Usage.PromptTokens, chunk.Usage.CompletionTokens, chunk.Usage.Cost)

			statistics = CreateStatistics(chunk.Model, provider, chunk.Usage)
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]
		delta := choice.Delta

		if choice.FinishReason != "" {
			finish = choice.FinishReason
		}

		calls := delta.ToolCalls

		if len(calls) > 0 {
			call := calls[0]

			if open > 0 && open == close {
				continue
			}

			if tool == nil {
				tool = &ChatToolCall{}
			}

			if call.ID != "" && !strings.HasSuffix(tool.ID, call.ID) {
				tool.ID += call.ID
			}

			if call.Function != nil {
				if call.Function.Name != "" && !strings.HasSuffix(tool.Name, call.Function.Name) {
					tool.Name += call.Function.Name
				}

				open += strings.Count(call.Function.Arguments, "{")
				close += strings.Count(call.Function.Arguments, "}")

				tool.Args += call.Function.Arguments
			}

			if len(delta.ReasoningDetails) != 0 && tool.Reasoning == nil {
				for _, details := range delta.ReasoningDetails {
					if details.Type != openingrouter.ChatReasoningDetailTypeEncrypted {
						continue
					}

					tool.Reasoning = &ChatToolReasoning{
						Format:    string(details.Format),
						Encrypted: details.Data,
					}
				}
			}

			markToken(true)

			hasContent = true
		}

		if delta.Content != "" {
			if !completing {
				delta.Content = strings.TrimLeft(delta.Content, " \t\n\r")

				if delta.Content == "" {
					continue
				} else {
					completing = true
				}
			}

			buf.WriteString(delta.Content)

			response.WriteChunk(NewChunk(ChunkText, delta.Content))

			markToken(true)

			hasContent = true
		} else if delta.Reasoning != "" {
			reasoningText := delta.Reasoning

			if !reasoning && len(delta.ReasoningDetails) != 0 {
				reasoningText = strings.TrimLeft(reasoningText, " \t\n\r")

				reasoning = true

				response.WriteChunk(NewChunk(ChunkReasoningType, delta.ReasoningDetails[0].Type))
			}

			response.WriteChunk(NewChunk(ChunkReasoning, reasoningText))

			if reasoningStart == 0 {
				reasoningStart = time.Since(started).Milliseconds()
			}

			markToken(false)
		} else if len(delta.Images) > 0 {
			for _, image := range delta.Images {
				if image.ImageURL.URL == "" {
					continue
				}

				response.WriteChunk(NewChunk(ChunkImage, image.ImageURL.URL))

				outputImages++

				markToken(true)

				hasContent = true
			}
		}
	}

	badStop := GetBadStopReason(finish, native)
	if badStop != "" {
		response.WriteChunk(NewChunk(ChunkError, fmt.Errorf("stopped due to: %s", badStop)))
	}

	noContent := buf.Len() == 0 && finish == "" && !hasContent
	if noContent {
		response.WriteChunk(NewChunk(ChunkError, errors.New("no content returned")))
	}

	if statistics != nil {
		response.WriteChunk(NewChunk(ChunkUsage, *statistics))
	}

	return tool, buf.String(), nil
}

func GetBadStopReason(finish openingrouter.ChatFinishReason, native string) string {
	if finish == "" {
		return ""
	}

	switch finish {
	case openingrouter.ChatFinishReasonLength:
		return "token limit reached"
	case openingrouter.ChatFinishReasonContentFilter:
		return "content filter"
	}

	debug("finished with: %q", finish)

	if native == "" {
		return ""
	}

	mapped, ok := nativeFinishReasons[native]
	if ok {
		return mapped
	}

	debug("unknown native finish reason: %q", native)

	return ""
}

func LimitChatRequestImages(request *openingrouter.ChatCompletionRequest, maxImages int) {
	images, _ := countMediaInRequest(request)
	toRemove := images - maxImages

	if toRemove <= 0 {
		return
	}

	for i := range request.Messages {
		message := &request.Messages[i]
		parts := message.Content.Parts[:0]

		for _, part := range message.Content.Parts {
			if toRemove > 0 && part.Type == openingrouter.ChatContentPartTypeImageURL {
				toRemove--

				continue
			}

			parts = append(parts, part)
		}

		message.Content.Parts = parts

		if toRemove == 0 || len(message.Images) == 0 {
			continue
		}

		remove := min(toRemove, len(message.Images))
		message.Images = message.Images[remove:]
		toRemove -= remove

		if toRemove == 0 {
			return
		}
	}
}

func countMediaInRequest(request *openingrouter.ChatCompletionRequest) (int, int) {
	var (
		images int
		files  int
	)

	for _, message := range request.Messages {
		images += len(message.Images)

		for _, part := range message.Content.Parts {
			switch part.Type {
			case openingrouter.ChatContentPartTypeImageURL:
				images++
			case openingrouter.ChatContentPartTypeFile:
				files++
			case openingrouter.ChatContentPartTypeText:
				files += strings.Count(part.Text, "<file name=")
			}
		}

		if message.Content.Text != "" {
			files += strings.Count(message.Content.Text, "<file name=")
		}
	}

	return images, files
}
