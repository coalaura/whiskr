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
	Proxy string `json:"proxy"`
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

	proxy, err := ResolveProxy(req.Proxy)
	if err != nil {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
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

	client := OpenRouterClient(proxy)

	speechReq := openrouter.SpeechRequest{
		Model: req.Model,
		Input: req.Input,
		Voice: req.Voice,
	}

	format, ok := AudioFormats[req.Model]
	if ok {
		speechReq.ResponseFormat = format.Optimal
	}

	debug("requesting %s speech from %q", speechReq.ResponseFormat, speechReq.Model)

	resp, err := client.CreateSpeech(ctx, speechReq)
	if err != nil {
		stream.WriteChunk(NewChunk(ChunkError, err.Error()))

		return
	}

	audioData, contentType := processAudio(speechReq.ResponseFormat, resp.ContentType, resp.Audio)

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

func isMP3Audio(audio []byte) bool {
	if len(audio) < 3 {
		return false
	}

	if audio[0] == 'I' && audio[1] == 'D' && audio[2] == '3' {
		return true
	}

	return audio[0] == 0xFF && audio[1]&0xE0 == 0xE0
}

func detectAudioFormat(requestedFormat openrouter.SpeechResponseFormat, contentType string, audio []byte) string {
	if isWAVAudio(audio) {
		return "wav"
	}

	if isMP3Audio(audio) {
		return "mp3"
	}

	switch requestedFormat {
	case "mp3", "wav", "pcm":
		return string(requestedFormat)
	}

	ct := strings.ToLower(contentType)

	switch {
	case strings.Contains(ct, "mpeg") || strings.Contains(ct, "mp3"):
		return "mp3"
	case strings.Contains(ct, "wav"):
		return "wav"
	case strings.Contains(ct, "pcm"):
		return "pcm"
	}

	return ""
}

func processAudio(requestedFormat openrouter.SpeechResponseFormat, contentType string, audio []byte) ([]byte, string) {
	detected := detectAudioFormat(requestedFormat, contentType, audio)

	switch detected {
	case "mp3":
		return audio, "audio/mpeg"
	case "wav":
		return audio, "audio/wav"
	case "pcm":
		var buf bytes.Buffer

		err := writeWAVHeader(&buf, len(audio), contentType)
		if err == nil {
			buf.Write(audio)

			return buf.Bytes(), "audio/wav"
		}

		return audio, "audio/pcm"
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return audio, contentType
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
