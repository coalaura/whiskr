package main

import (
	"fmt"
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

func SplitImagePairs(text string, stripImages bool) []openrouter.ChatMessagePart {
	code := FindMarkdownCodeRegions(text)

	rgx := regexp.MustCompile(`(?m)!\[([^\]]*)\]\((\S+?)\)`)

	var (
		index int
		parts []openrouter.ChatMessagePart
	)

	push := func(str, end int, suffix string) {
		if str > end {
			return
		}

		rest := text[str:end] + suffix

		total := len(parts)

		if total > 0 && parts[total-1].Type == openrouter.ChatMessagePartTypeText {
			parts[total-1].Text += rest

			return
		}

		rest = strings.TrimSpace(rest)

		if rest == "" {
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
			push(index, len(text), "")

			break
		}

		start := index + location[0]
		end := index + location[1]

		if IsInsideCodeBlock(start, code) {
			push(index, end, "")

			index = end

			continue
		}

		altStart := index + location[2]
		altEnd := index + location[3]

		urlStart := index + location[4]
		urlEnd := index + location[5]

		alt := text[altStart:altEnd]
		url := text[urlStart:urlEnd]

		isDataUrl := strings.HasPrefix(url, "data:")
		isHttpUrl := strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://")

		if !isDataUrl && !isHttpUrl {
			push(index, end, "")

			index = end

			continue
		}

		var image string

		if isDataUrl {
			image = fmt.Sprintf("![image](%s)", alt)
		} else {
			image = fmt.Sprintf("![image](%s)", url)
		}

		if start > index {
			push(index, start, image)
		} else {
			push(index, index, image)
		}

		if !stripImages {
			parts = append(parts, openrouter.ChatMessagePart{
				Type: openrouter.ChatMessagePartTypeImageURL,
				ImageURL: &openrouter.ChatMessageImageURL{
					Detail: openrouter.ImageURLDetailAuto,
					URL:    url,
				},
			})
		}

		index = end
	}

	return parts
}
