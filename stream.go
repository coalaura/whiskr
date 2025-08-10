package main

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/revrost/go-openrouter"
)

type Chunk struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Stream struct {
	wr http.ResponseWriter
	fl http.Flusher
	en *json.Encoder
}

func NewStream(w http.ResponseWriter) (*Stream, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, errors.New("failed to create flusher")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	return &Stream{
		wr: w,
		fl: flusher,
		en: json.NewEncoder(w),
	}, nil
}

func (s *Stream) Send(ch Chunk) error {
	if err := s.en.Encode(ch); err != nil {
		return err
	}

	if _, err := s.wr.Write([]byte("\n\n")); err != nil {
		return err
	}

	s.fl.Flush()

	return nil
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
		Text: text,
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
