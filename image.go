package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
)

type ImageRequest struct {
	Name    string
	Content string
}

func HandleImage(w http.ResponseWriter, r *http.Request) {
	debug("parsing image")

	request, err := ReadImageRequest(r)
	if err != nil {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})

		return
	}

	mime, data, err := DecodeDataURL(request.Content)
	if err != nil {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})

		return
	}

	name := strings.TrimSpace(request.Name)
	if name == "" {
		name = "image"
	}

	name = strings.ReplaceAll(name, "\r", "")
	name = strings.ReplaceAll(name, "\n", "")

	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%s", strconv.Quote(name)))
	w.WriteHeader(http.StatusOK)

	w.Write(data)
}

func ReadImageRequest(r *http.Request) (*ImageRequest, error) {
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, err
	}

	var request ImageRequest

	for {
		part, err := reader.NextPart()
		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, err
		}

		switch part.FormName() {
		case "name":
			request.Name, err = ReadMultipartPart(part)
		case "content":
			request.Content, err = ReadMultipartPart(part)
		}

		if err != nil {
			return nil, err
		}
	}

	if request.Content == "" {
		return nil, errors.New("missing image content")
	}

	return &request, nil
}

func ReadMultipartPart(part *multipart.Part) (string, error) {
	b, err := io.ReadAll(part)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func DecodeDataURL(dataURL string) (string, []byte, error) {
	if !strings.HasPrefix(dataURL, "data:") {
		return "", nil, errors.New("invalid data url")
	}

	comma := strings.IndexByte(dataURL, ',')
	if comma == -1 {
		return "", nil, errors.New("invalid data url")
	}

	meta := dataURL[5:comma]
	if !strings.HasSuffix(meta, ";base64") {
		return "", nil, errors.New("unsupported image encoding")
	}

	mime := strings.TrimSuffix(meta, ";base64")
	if !strings.HasPrefix(mime, "image/") {
		return "", nil, errors.New("invalid image mime type")
	}

	raw := dataURL[comma+1:]

	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", nil, err
	}

	return mime, data, nil
}
