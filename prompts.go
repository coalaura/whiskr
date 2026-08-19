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

	"github.com/coalaura/whiskr/internal/desktop"
)

type PromptData struct {
	Name     string
	Slug     string
	Date     string
	Platform string
	Settings ChatSettings
}

type Prompt struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Tokens      int    `json:"tokens"`

	Text string `json:"-"`
	Body string `json:"-"`
}

var (
	//go:embed internal/tools.txt
	InternalToolsPrompt string

	InternalToolsTmpl *template.Template

	//go:embed internal/general.txt
	InternalGeneralPrompt string

	//go:embed internal/files.txt
	InternalFilesPrompt string

	//go:embed internal/no_files.txt
	InternalNoFilesPrompt string

	//go:embed internal/title.txt
	InternalTitlePrompt string

	InternalTitleTmpl *template.Template

	Prompts       []Prompt
	Templates     = make(map[string]*template.Template)
	BareTemplates = make(map[string]*template.Template)

	PromptsPath = desktop.ResolveRelativePath("prompts")
)

func init() {
	InternalToolsTmpl = NewTemplate("internal-tools", InternalToolsPrompt)
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

	err := filepath.Walk(PromptsPath, func(path string, info fs.FileInfo, err error) error {
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
			log.Warnf("Invalid prompt file (no delimiter): %q\n", path)

			return nil
		}

		nl := bytes.Index(body[:index], []byte("\n"))
		if nl == -1 {
			log.Warnf("Invalid prompt file (no description): %q\n", path)

			return nil
		}

		bodyText := strings.TrimSpace(string(body[index+3:]))

		prompt := Prompt{
			Key:         strings.Replace(filepath.Base(path), ".txt", "", 1),
			Name:        strings.TrimSpace(string(body[:nl])),
			Description: strings.TrimSpace(string(body[nl+1 : index])),
			Body:        bodyText,
			Text:        bodyText + "\n\n" + InternalGeneralPrompt,
		}

		prompts = append(prompts, prompt)

		Templates[prompt.Key] = NewTemplate(prompt.Key, prompt.Text)
		BareTemplates[prompt.Key] = NewTemplate(prompt.Key+"-bare", prompt.Body)

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

func BuildPrompt(name string, metadata ChatMetadata, model *Model, bare bool) (string, error) {
	if name == "" {
		return "", nil
	}

	templates := Templates
	if bare {
		templates = BareTemplates
	}

	tmpl, ok := templates[name]
	if !ok {
		return "", fmt.Errorf("unknown prompt: %q", name)
	}

	if metadata.Platform == "" {
		metadata.Platform = "Unknown"
	}

	settings := metadata.Settings
	if bare {
		settings = ChatSettings{}
	}

	buf := GetFreeBuffer()
	defer pool.Put(buf)

	err := tmpl.Execute(buf, PromptData{
		Name:     model.Name,
		Slug:     model.Slug,
		Date:     FormatPromptDate(metadata),
		Platform: metadata.Platform,
		Settings: settings,
	})

	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func FormatPromptDate(metadata ChatMetadata) string {
	tz := time.UTC

	if metadata.Timezone != "" {
		if parsed, err := time.LoadLocation(metadata.Timezone); err == nil {
			tz = parsed
		}
	}

	now := time.Now()
	if metadata.Time != nil {
		now = time.Unix(*metadata.Time, 0)
	}

	return now.In(tz).Format(time.RFC1123)
}
