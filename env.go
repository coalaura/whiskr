package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

var (
	Debug           bool
	MaxIterations   int
	OpenRouterToken string
)

func init() {
	log.MustPanic(godotenv.Load())

	Debug = os.Getenv("DEBUG") == "true"

	if env := os.Getenv("MAX_ITERATIONS"); env != "" {
		iterations, err := strconv.Atoi(env)
		if err != nil {
			log.Panic(fmt.Errorf("invalid max iterations: %v", err))
		}

		if iterations < 1 {
			log.Panic(errors.New("max iterations has to be 1 or more"))
		}

		MaxIterations = iterations
	} else {
		MaxIterations = 3
	}

	if OpenRouterToken = os.Getenv("OPENROUTER_TOKEN"); OpenRouterToken == "" {
		log.Panic(errors.New("missing openrouter token"))
	}

	if Debug {
		log.Warning("Debug mode enabled")
	}
}
