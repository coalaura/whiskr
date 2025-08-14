package main

import "strings"

var cleaner = strings.NewReplacer(
	"‑", "-",
	"—", "-",

	"“", "\"",
	"”", "\"",
	"’", "'",
)

func CleanChunk(chunk string) string {
	if !CleanContent {
		return chunk
	}

	return cleaner.Replace(chunk)
}
