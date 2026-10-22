package main

import (
	"context"
	"errors"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/coalaura/openingrouter"
	"github.com/coalaura/whiskr/internal/paths"
)

func init() {
	log.SetTarget(io.Discard)

	var err error

	path, err = paths.ResolvePaths()
	if err != nil {
		panic(err)
	}

	env, err = LoadEnv()
	if err != nil {
		panic(err)
	}
}

func TestProviderIcons(t *testing.T) {
	registry, err := LoadProviderRegistry(context.Background())
	if err != nil {
		t.Fatal(err)

		return
	}

	seen := make(map[string]struct{})

	for name, provider := range registry {
		icon := filepath.Join("static", "public", provider.Icon)
		base := filepath.Base(icon)

		if _, ok := seen[base]; ok {
			continue
		}

		seen[base] = struct{}{}

		checkIcon(t, name, icon)
	}

	t.Logf("checked %d providers\n", len(registry))
}

func TestLabIcons(t *testing.T) {
	list, err := openingrouter.ListFrontendModels(context.Background())
	if err != nil {
		t.Fatal(err)

		return
	}

	dir := filepath.Join("static", "public", "labs")

	seen := make(map[string]struct{})

	for _, model := range list {
		lab := model.Author

		base := model.Author + ".png"
		icon := filepath.Join(dir, base)

		if _, ok := seen[base]; ok {
			continue
		}

		seen[base] = struct{}{}

		checkIcon(t, lab, icon)
	}

	t.Logf("checked %d labs\n", len(seen))
}

func checkIcon(t *testing.T, name, icon string) {
	t.Helper()

	file, err := os.Open(icon)
	if err != nil {
		if os.IsNotExist(err) {
			err = errors.New("not found")
		}

		t.Errorf("%s (%s): %s", name, icon, err)

		return
	}

	defer file.Close()

	config, err := png.DecodeConfig(file)
	if err != nil {
		t.Errorf("%s (%s): %s", name, icon, err)

		return
	}

	if config.Width != 128 || config.Height != 128 {
		t.Errorf("%s (%s): got %dx%d, want 128x128", name, icon, config.Width, config.Height)
	}
}
