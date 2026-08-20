//go:build desktop && release

package paths

import (
	"os"
	"path/filepath"
)

func ResolvePaths() (Paths, error) {
	config, err := ensureConfigDir()
	if err != nil {
		return Paths{}, err
	}

	cache, err := ensureCacheDir()
	if err != nil {
		return Paths{}, err
	}

	exe, err := getExecutableDir()
	if err != nil {
		return Paths{}, err
	}

	return Paths{
		Config:          filepath.Join(config, "config.yml"),
		Settings:        filepath.Join(config, "settings.yml"),
		Prompts:         filepath.Join(exe, "prompts"),
		VocabularyCache: filepath.Join(cache, "vocabulary.tiktoken"),
	}, nil
}

func ensureConfigDir() (string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	config = filepath.Join(config, "whiskr")

	err = os.MkdirAll(config, 0755)
	if err != nil {
		return "", err
	}

	return config, nil
}

func ensureCacheDir() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}

	cache = filepath.Join(cache, "whiskr")

	err = os.MkdirAll(cache, 0755)
	if err != nil {
		return "", err
	}

	return cache, nil
}

func getExecutableDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}

	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}

	return filepath.Dir(exe), nil
}
