package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"

	"github.com/revrost/go-openrouter"
)

type Chunk struct {
	Type string `json:"type"`
	Text any    `json:"text,omitempty"`
}

type Stream struct {
	wr  http.ResponseWriter
	ctx context.Context
}

var pool = sync.Pool{
	New: func() interface{} {
		return &bytes.Buffer{}
	},
}

func NewStream(w http.ResponseWriter, ctx context.Context) (*Stream, error) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	return &Stream{
		wr:  w,
		ctx: ctx,
	}, nil
}

func (s *Stream) Send(ch Chunk) error {
	debugIf(ch.Type == "error", "error: %v", ch.Text)

	return WriteChunk(s.wr, s.ctx, ch)
}

func ReasoningChunk(text string) Chunk {
	return Chunk{
		Type: "reason",
		Text: text,
	}
}

func TextChunk(text string) Chunk {
	return Chunk{
		Type: "text",
		Text: CleanChunk(text),
	}
}

func ToolChunk(tool *ToolCall) Chunk {
	return Chunk{
		Type: "tool",
		Text: tool,
	}
}

func IDChunk(id string) Chunk {
	return Chunk{
		Type: "id",
		Text: id,
	}
}

func EndChunk() Chunk {
	return Chunk{
		Type: "end",
	}
}

func StartChunk() Chunk {
	return Chunk{
		Type: "start",
	}
}

func ErrorChunk(err error) Chunk {
	return Chunk{
		Type: "error",
		Text: GetErrorMessage(err),
	}
}

func GetErrorMessage(err error) string {
	if apiErr, ok := err.(*openrouter.APIError); ok {
		return apiErr.Error()
	}

	return err.Error()
}

func WriteChunk(w http.ResponseWriter, ctx context.Context, chunk any) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	buf := pool.Get().(*bytes.Buffer)

	buf.Reset()

	defer pool.Put(buf)

	if err := json.NewEncoder(buf).Encode(chunk); err != nil {
		return err
	}

	buf.Write([]byte("\n\n"))

	if _, err := w.Write(buf.Bytes()); err != nil {
		return err
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		return errors.New("failed to create flusher")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		flusher.Flush()

		return nil
	}
}
