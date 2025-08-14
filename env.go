package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

var (
	Debug bool

	CleanContent  bool
	MaxIterations int

	OpenRouterToken string
	ExaToken        string
)

func init() {
	log.MustPanic(godotenv.Load())

	// enable debug logs & prints
	Debug = os.Getenv("DEBUG") == "true"

	if Debug {
		log.Warning("Debug mode enabled")
	}

	// de-ai assistant response content
	CleanContent = os.Getenv("DEBUG") == "true"

	// maximum amount of iterations per turn
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

	// openrouter token used for all completions & model list
	if OpenRouterToken = os.Getenv("OPENROUTER_TOKEN"); OpenRouterToken == "" {
		log.Panic(errors.New("missing openrouter token"))
	}

	// optional exa token used for search tools
	if ExaToken = os.Getenv("EXA_TOKEN"); ExaToken == "" {
		log.Warning("missing exa token, web search unavailable")
	}
}
