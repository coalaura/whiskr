package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func RunConfigCommand(args []string) error {
	if len(args) > 1 {
		return errors.New("expected init [settings.yml]")
	}

	var input string

	if len(args) > 0 {
		input = args[0]
	}

	log.SetTarget(io.Discard)

	return createInitialConfig(input)
}

func createInitialConfig(input string) error {
	cfg := DefaultEnv()

	if input != "" {
		var err error

		cfg, err = ReadEnv(input)
		if err != nil {
			return fmt.Errorf("read initial configuration: %w", err)
		}

		err = cfg.Init(false)
		if err != nil {
			return fmt.Errorf("validate initial configuration: %w", err)
		}
	} else {
		cfg.applyDefaults()
	}

	body, err := cfg.configYAML()
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(path.Config), 0755)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(path.Config, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}

	_, err = file.Write(body)
	closeErr := file.Close()

	if err != nil || closeErr != nil {
		os.Remove(path.Config)

		return errors.Join(err, closeErr)
	}

	return nil
}
