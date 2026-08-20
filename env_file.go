package main

import (
	_ "embed"
	"errors"
	"os"
	"path/filepath"
)

//go:embed example.config.yml
var exampleConfig []byte

func EnsureConfig(name string) (bool, error) {
	_, err := os.Stat(name)
	if err == nil {
		return false, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}

	err = os.MkdirAll(filepath.Dir(name), 0755)
	if err != nil {
		return false, err
	}

	file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	var written bool

	defer func() {
		file.Close()

		if !written {
			os.Remove(name)
		}
	}()

	_, err = file.Write(exampleConfig)
	if err != nil {
		return false, err
	}

	err = file.Close()
	if err != nil {
		return false, err
	}

	written = true

	return true, nil
}
