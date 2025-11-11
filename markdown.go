package main

import (
	"regexp"
	"strings"

	"github.com/revrost/go-openrouter"
)

type CodeRegion struct {
	Start int
	End   int
}

func FindMarkdownCodeRegions(text string) []CodeRegion {
	var regions []CodeRegion

	inline := regexp.MustCompile(`\x60[^\x60\n]+?\x60`)

	for _, match := range inline.FindAllStringIndex(text, -1) {
		regions = append(regions, CodeRegion{
			Start: match[0],
			End:   match[1],
		})
	}

	fenced := regexp.MustCompile(`(?m)^\x60\x60\x60[^\n]*\n(.*?\n)^\x60\x60\x60\s*$`)

	for _, match := range fenced.FindAllStringIndex(text, -1) {
		regions = append(regions, CodeRegion{
			Start: match[0],
			End:   match[1],
		})
	}

	return regions
}

func IsInsideCodeBlock(pos int, regions []CodeRegion) bool {
	for _, region := range regions {
		if pos >= region.Start && pos < region.End {
			return true
		}
	}

	return false
}

func SplitImagePairs(text string) []openrouter.ChatMessagePart {
	code := FindMarkdownCodeRegions(text)

	rgx := regexp.MustCompile(`(?m)!\[[^\]]*]\((\S+?)\)`)

	var (
		index int
		parts []openrouter.ChatMessagePart
	)

	push := func(str, end int) {
		if str > end {
			return
		}

		rest := text[str:end]

		total := len(parts)

		if total > 0 && parts[total-1].Type == openrouter.ChatMessagePartTypeText {
			parts[total-1].Text += rest

			return
		}

		if strings.TrimSpace(rest) == "" {
			return
		}

		parts = append(parts, openrouter.ChatMessagePart{
			Type: openrouter.ChatMessagePartTypeText,
			Text: rest,
		})
	}

	for {
		location := rgx.FindStringSubmatchIndex(text[index:])
		if location == nil {
			push(index, len(text))

			break
		}

		start := index + location[0]
		end := index + location[1]

		if IsInsideCodeBlock(start, code) {
			push(index, end)

			index = end

			continue
		}

		urlStart := index + location[2]
		urlEnd := index + location[3]

		url := text[urlStart:urlEnd]

		if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
			push(index, end)

			index = end

			continue
		}

		if start > index {
			push(index, start)
		}

		parts = append(parts, openrouter.ChatMessagePart{
			Type: openrouter.ChatMessagePartTypeImageURL,
			ImageURL: &openrouter.ChatMessageImageURL{
				Detail: openrouter.ImageURLDetailAuto,
				URL:    url,
			},
		})

		index = end
	}

	return parts
}
