package main

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

var OpenRouterToken string

func init() {
	log.MustPanic(godotenv.Load())

	if OpenRouterToken = os.Getenv("OPENROUTER_TOKEN"); OpenRouterToken == "" {
		log.Panic(errors.New("missing openrouter token"))
	}
}
