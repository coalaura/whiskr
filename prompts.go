package main

import (
	"bytes"
	_ "embed"
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
	Name     string
	Slug     string
	Date     string
	Platform string
}

type Prompt struct {
	Key  string `json:"key"`
	Name string `json:"name"`

	Text string `json:"-"`
}

var (
	//go:embed internal/tools.txt
	InternalToolsPrompt string

	//go:embed internal/title.txt
	InternalTitlePrompt string

	InternalTitleTmpl *template.Template

	Prompts   []Prompt
	Templates = make(map[string]*template.Template)
)

func init() {
	InternalTitleTmpl = NewTemplate("internal-title", InternalTitlePrompt)

	var err error

	Prompts, err = LoadPrompts()
	log.MustFail(err)
}

func NewTemplate(name, text string) *template.Template {
	text = strings.ReplaceAll(text, "\r", "")

	return template.Must(template.New(name).Parse(text))
}

func LoadPrompts() ([]Prompt, error) {
	var prompts []Prompt

	log.Println("Loading prompts...")

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
			log.Warnf("Invalid prompt file: %q\n", path)

			return nil
		}

		prompt := Prompt{
			Key:  strings.Replace(filepath.Base(path), ".txt", "", 1),
			Name: strings.TrimSpace(string(body[:index])),
			Text: strings.TrimSpace(string(body[index+3:])),
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

	log.Printf("Loaded %d prompts\n", len(prompts))

	return prompts, nil
}

func BuildPrompt(name string, metadata Metadata, model *Model) (string, error) {
	if name == "" {
		return "", nil
	}

	tmpl, ok := Templates[name]
	if !ok {
		return "", fmt.Errorf("unknown prompt: %q", name)
	}

	tz := time.UTC

	if metadata.Timezone != "" {
		parsed, err := time.LoadLocation(metadata.Timezone)
		if err == nil {
			tz = parsed
		}
	}

	if metadata.Platform == "" {
		metadata.Platform = "Unknown"
	}

	buf := GetFreeBuffer()
	defer pool.Put(buf)

	err := tmpl.Execute(buf, PromptData{
		Name:     model.Name,
		Slug:     model.ID,
		Date:     time.Now().In(tz).Format(time.RFC1123),
		Platform: metadata.Platform,
	})

	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
