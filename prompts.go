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

	//go:embed prompts/reviewer.txt
	PromptReviewer string

	//go:embed prompts/engineer.txt
	PromptEngineer string

	//go:embed prompts/scripts.txt
	PromptScripts string

	//go:embed prompts/physics.txt
	PromptPhysics string

	Templates = map[string]*template.Template{
		"normal":   NewTemplate("normal", PromptNormal),
		"reviewer": NewTemplate("reviewer", PromptReviewer),
		"engineer": NewTemplate("engineer", PromptEngineer),
		"scripts":  NewTemplate("scripts", PromptScripts),
		"physics":  NewTemplate("physics", PromptPhysics),
	}
)

func NewTemplate(name, text string) *template.Template {
	return template.Must(template.New(name).Parse(text))
}

func BuildPrompt(name string, model *Model) (string, error) {
	if name == "" {
		return "", nil
	}

	tmpl, ok := Templates[name]
	if !ok {
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
