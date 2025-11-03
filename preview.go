package main

import (
	_ "embed"
	"encoding/json"
	"html/template"
	"io"
	"mime/multipart"
	"net/http"
)

var (
	//go:embed internal/preview.html
	InternalPreview string

	InternalPreviewTmpl *template.Template
)

type PreviewRequest struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

func init() {
	InternalPreviewTmpl = template.Must(template.New("internal-preview").Funcs(template.FuncMap{
		"json": func(val any) template.JS {
			b, err := json.Marshal(val)
			if err != nil {
				return template.JS("null")
			}

			return template.JS(b)
		},
	}).Parse(InternalPreview))
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

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)

	InternalPreviewTmpl.Execute(w, request)
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
