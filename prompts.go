package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"
	"time"
)

type PromptData struct {
	Name string
	Slug string
	Date string
}

var (
	//go:embed prompts/normal.txt
	PromptNormal string

	PromptNormalTmpl = template.Must(template.New("normal").Parse(PromptNormal))
)

func BuildPrompt(name string, model *Model) (string, error) {
	if name == "" {
		return "", nil
	}

	var tmpl *template.Template

	switch name {
	case "normal":
		tmpl = PromptNormalTmpl
	default:
		return "", fmt.Errorf("unknown prompt: %q", name)
	}

	var buf bytes.Buffer

	err := tmpl.Execute(&buf, PromptData{
		Name: model.Name,
		Slug: model.ID,
		Date: time.Now().Format(time.RFC1123),
	})

	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
