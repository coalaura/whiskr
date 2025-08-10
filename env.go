package main

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

var (
	Debug           bool
	NoOpen          bool
	OpenRouterToken string
)

func init() {
	log.MustPanic(godotenv.Load())

	Debug = os.Getenv("DEBUG") == "true"
	NoOpen = os.Getenv("NO_OPEN") == "true"

	if OpenRouterToken = os.Getenv("OPENROUTER_TOKEN"); OpenRouterToken == "" {
		log.Panic(errors.New("missing openrouter token"))
	}

	if Debug {
		log.Warning("Debug mode enabled")
	}
}
