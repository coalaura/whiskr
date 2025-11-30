package main

import "strings"

var cleaner = strings.NewReplacer(
	"‑", "-",
	"—", "-",
	"–", "-",
	"•", "-",

	"“", "\"",
	"”", "\"",
	"’", "'",
)

func CleanChunk(chunk string) string {
	if !env.Settings.CleanContent {
		return chunk
	}

	return cleaner.Replace(chunk)
}
