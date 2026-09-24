package main

import (
	"context"
	"errors"
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

		_, err = os.Stat(icon)
		if err == nil {
			continue
		}

		if os.IsNotExist(err) {
			err = errors.New("not found")
		}

		t.Errorf("%s (%s): %s\n", name, icon, err)
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

		_, err = os.Stat(icon)
		if err == nil {
			continue
		}

		if os.IsNotExist(err) {
			err = errors.New("not found")
		}

		t.Errorf("%s (%s): %s\n", lab, icon, err)
	}

	t.Logf("checked %d labs\n", len(seen))
}
