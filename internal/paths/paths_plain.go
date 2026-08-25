//go:build !desktop || !release

package paths

import (
	"os"
	"path/filepath"
)

func ResolvePaths() (Paths, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Paths{}, err
	}

	return Paths{
		Config:          filepath.Join(cwd, "config.yml"),
		Settings:        filepath.Join(cwd, "settings.yml"),
		Prompts:         filepath.Join(cwd, "prompts"),
		ProviderIcons:   filepath.Join(cwd, "providers"),
		VocabularyCache: filepath.Join(cwd, "vocabulary.tiktoken"),
	}, nil
}
