package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net/http"
	"sync"

	"github.com/revrost/go-openrouter"
	"github.com/vmihailenco/msgpack/v5"
)

const (
	ChunkStart         ChunkType = 0
	ChunkID            ChunkType = 1
	ChunkReasoning     ChunkType = 2
	ChunkReasoningType ChunkType = 3
	ChunkText          ChunkType = 4
	ChunkImage         ChunkType = 5
	ChunkTool          ChunkType = 6
	ChunkError         ChunkType = 7
	ChunkEnd           ChunkType = 8
	ChunkAlive         ChunkType = 9
)

type ChunkType uint8

type Chunk struct {
	Type ChunkType
	Data any
}

type StartChunk struct {
	Iteration int64 `msgpack:"iteration"`
	Total     int64 `msgpack:"total"`
}

type Stream struct {
	mx  sync.Mutex
	wr  http.ResponseWriter
	ctx context.Context
}

var pool = sync.Pool{
	New: func() any {
		return &bytes.Buffer{}
	},
}

func GetFreeBuffer() *bytes.Buffer {
	buf := pool.Get().(*bytes.Buffer)

	buf.Reset()

	return buf
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

func NewChunk(typ ChunkType, data any) *Chunk {
	if str, ok := data.(string); ok {
		data = CleanChunk(str)
	} else if err, ok := data.(error); ok {
		data = err.Error()
	}

	return &Chunk{
		Type: typ,
		Data: data,
	}
}

func GetErrorMessage(err error) string {
	if apiErr, ok := err.(*openrouter.APIError); ok {
		return apiErr.Error()
	}

	return err.Error()
}

func (s *Stream) WriteChunk(chunk *Chunk) error {
	s.mx.Lock()
	defer s.mx.Unlock()

	debugIf(chunk.Type == ChunkError, "error: %v", chunk.Data)

	if err := s.ctx.Err(); err != nil {
		return err
	}

	buf := GetFreeBuffer()
	defer pool.Put(buf)

	binary.Write(buf, binary.LittleEndian, chunk.Type)

	if chunk.Data != nil {
		data, err := msgpack.Marshal(chunk.Data)
		if err != nil {
			return err
		}

		binary.Write(buf, binary.LittleEndian, uint32(len(data)))

		buf.Write(data)
	} else {
		binary.Write(buf, binary.LittleEndian, uint32(0))
	}

	if _, err := s.wr.Write(buf.Bytes()); err != nil {
		return err
	}

	flusher, ok := s.wr.(http.Flusher)
	if !ok {
		return errors.New("failed to create flusher")
	}

	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	default:
		flusher.Flush()

		return nil
	}
}
