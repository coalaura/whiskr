package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/revrost/go-openrouter"
)

type TTSRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
	Voice string `json:"voice"`
}

type TTSResponseChunk struct {
	Audio       []byte `msgpack:"audio"`
	ContentType string `msgpack:"content_type"`
}

const (
	DefaultTTSPCMSampleRate = 24000
	DefaultTTSPCMChannels   = 1
	DefaultTTSPCMBits       = 16
)

func HandleTTS(w http.ResponseWriter, r *http.Request) {
	if !env.Models.TextToSpeech {
		RespondJson(w, http.StatusForbidden, map[string]any{
			"error": "Text-to-speech is disabled",
		})

		return
	}

	var req TTSRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": "invalid request body",
		})

		return
	}

	req.Model = strings.TrimSpace(req.Model)
	req.Input = strings.TrimSpace(req.Input)

	if req.Input == "" {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": "input text is required",
		})

		return
	}

	if req.Model == "" {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": "model is required",
		})

		return
	}

	ctx := r.Context()

	stream, err := NewStream(w, ctx)
	if err != nil {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})

		return
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stream.WriteChunk(NewChunk(ChunkAlive, nil))
			}
		}
	}()

	client := OpenRouterClient()

	speechReq := openrouter.SpeechRequest{
		Model: req.Model,
		Input: req.Input,
		Voice: req.Voice,
	}

	resp, err := client.CreateSpeech(ctx, speechReq)
	if err != nil {
		stream.WriteChunk(NewChunk(ChunkError, err.Error()))

		return
	}

	contentType := resp.ContentType

	isPCM := strings.Contains(strings.ToLower(contentType), "pcm") && !isWAVAudio(resp.Audio)

	audioData := resp.Audio

	if isPCM {
		var buf bytes.Buffer

		err = writeWAVHeader(&buf, len(resp.Audio), resp.ContentType)
		if err == nil {
			buf.Write(resp.Audio)

			audioData = buf.Bytes()
		}

		contentType = "audio/wav"
	} else if contentType == "" {
		contentType = "audio/mpeg"
	}

	stream.WriteChunk(NewChunk(ChunkAudio, TTSResponseChunk{
		Audio:       audioData,
		ContentType: contentType,
	}))
}

func parseAudioParamInt(params map[string]string, key string, fallback int) int {
	if value, ok := params[key]; ok {
		number, err := strconv.Atoi(value)
		if err == nil && number > 0 {
			return number
		}
	}

	return fallback
}

func isWAVAudio(audio []byte) bool {
	if len(audio) < 12 {
		return false
	}

	return string(audio[0:4]) == "RIFF" && string(audio[8:12]) == "WAVE"
}

func writeWAVHeader(w io.Writer, dataLen int, contentType string) error {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		params = map[string]string{}
	}

	sampleRate := parseAudioParamInt(params, "rate", DefaultTTSPCMSampleRate)
	channels := parseAudioParamInt(params, "channels", DefaultTTSPCMChannels)
	bitsPerSample := parseAudioParamInt(params, "bits", DefaultTTSPCMBits)

	if bitsPerSample%8 != 0 {
		bitsPerSample = DefaultTTSPCMBits
	}

	blockAlign := channels * (bitsPerSample / 8)
	if blockAlign <= 0 {
		blockAlign = DefaultTTSPCMChannels * (DefaultTTSPCMBits / 8)
	}

	byteRate := sampleRate * blockAlign
	if byteRate <= 0 {
		byteRate = DefaultTTSPCMSampleRate * DefaultTTSPCMChannels * (DefaultTTSPCMBits / 8)
	}

	header := make([]byte, 44)

	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], uint32(36+dataLen))
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16)
	binary.LittleEndian.PutUint16(header[20:22], 1)
	binary.LittleEndian.PutUint16(header[22:24], uint16(channels))
	binary.LittleEndian.PutUint32(header[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(header[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(header[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(header[34:36], uint16(bitsPerSample))
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], uint32(dataLen))

	_, err = w.Write(header)
	return err
}
