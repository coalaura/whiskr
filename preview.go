package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
)

var (
	//go:embed static/internal/preview.html
	InternalPreview []byte
)

type PreviewRequest struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

func HandlePreview(w http.ResponseWriter, r *http.Request) {
	debug("parsing preview")

	request, err := ReadPreviewRequest(r)
	if err != nil {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})

		return
	}

	debug("rendering preview")

	var data bytes.Buffer

	data.WriteString("<script>const data = ")

	err = json.NewEncoder(&data).Encode(request)
	if err != nil {
		RespondJson(w, http.StatusInternalServerError, map[string]any{
			"error": err.Error(),
		})

		return
	}

	data.WriteString("</script>")

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)

	index := bytes.Index(InternalPreview, []byte("<head>")) + 6

	w.Write(InternalPreview[:index])
	w.Write(data.Bytes())
	w.Write(InternalPreview[index:])
}

func ReadPreviewRequest(r *http.Request) (*PreviewRequest, error) {
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, err
	}

	var request PreviewRequest

	for {
		part, err := reader.NextPart()
		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, err
		}

		if part.FormName() == "name" {
			request.Name, err = ReadFormPart(part)
		} else if part.FormName() == "content" {
			request.Content, err = ReadFormPart(part)
		}

		if err != nil {
			return nil, err
		}
	}

	return &request, nil
}

func ReadFormPart(part *multipart.Part) (string, error) {
	b, err := io.ReadAll(part)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
