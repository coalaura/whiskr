package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"
)

type PromptData struct {
	Name string
	Slug string
	Date string
}

type Prompt struct {
	Key  string `json:"key"`
	Name string `json:"name"`

	Text string `json:"-"`
}

var (
	Prompts   []Prompt
	Templates = make(map[string]*template.Template)
)

func init() {
	var err error

	Prompts, err = LoadPrompts()
	log.MustPanic(err)
}

func NewTemplate(name, text string) *template.Template {
	return template.Must(template.New(name).Parse(text))
}

func LoadPrompts() ([]Prompt, error) {
	var prompts []Prompt

	log.Info("Loading prompts...")

	err := filepath.Walk("prompts", func(path string, info fs.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		file, err := os.OpenFile(path, os.O_RDONLY, 0)
		if err != nil {
			return err
		}

		defer file.Close()

		body, err := io.ReadAll(file)
		if err != nil {
			return err
		}

		index := bytes.Index(body, []byte("---"))
		if index == -1 {
			log.Warningf("Invalid prompt file: %q\n", path)

			return nil
		}

		prompt := Prompt{
			Key:  strings.Replace(filepath.Base(path), ".txt", "", 1),
			Name: strings.TrimSpace(string(body[:index])),
			Text: strings.TrimSpace(string(body[:index+3])),
		}

		prompts = append(prompts, prompt)

		Templates[prompt.Key] = NewTemplate(prompt.Key, prompt.Text)

		return nil
	})

	if err != nil {
		return nil, err
	}

	sort.Slice(prompts, func(i, j int) bool {
		return prompts[i].Name < prompts[j].Name
	})

	log.Infof("Loaded %d prompts\n", len(prompts))

	return prompts, nil
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
