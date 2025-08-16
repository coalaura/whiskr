package main

import (
	"errors"
	"os"

	"github.com/goccy/go-yaml"
)

type EnvTokens struct {
	OpenRouter string `json:"openrouter"`
	Exa        string `json:"exa"`
}

type EnvSettings struct {
	CleanContent  bool `json:"cleanup"`
	MaxIterations uint `json:"iterations"`
}

type Environment struct {
	Debug    bool        `json:"debug"`
	Tokens   EnvTokens   `json:"tokens"`
	Settings EnvSettings `json:"settings"`
}

var env Environment

func init() {
	file, err := os.OpenFile("config.yml", os.O_RDONLY, 0)
	log.MustPanic(err)

	defer file.Close()

	err = yaml.NewDecoder(file).Decode(&env)
	log.MustPanic(err)

	log.MustPanic(env.Init())
}

func (e *Environment) Init() error {
	// print if debug is enabled
	if e.Debug {
		log.Warning("Debug mode enabled")
	}

	// check max iterations
	e.Settings.MaxIterations = max(e.Settings.MaxIterations, 1)

	// check if openrouter token is set
	if e.Tokens.OpenRouter == "" {
		return errors.New("missing tokens.openrouter")
	}

	// check if exa token is set
	if e.Tokens.Exa == "" {
		log.Warning("missing token.exa, web search unavailable")
	}

	return nil
}
