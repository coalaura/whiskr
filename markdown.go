package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/coalaura/openingrouter"
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

	fenced := regexp.MustCompile("(?ms)^```[^\\n]*\\n.*?(?:^```[ \\t]*$|\\z)|^~~~[^\\n]*\\n.*?(?:^~~~[ \\t]*$|\\z)")

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

func SplitMediaPairs(text string, model *Model) []openingrouter.ChatContentPart {
	code := FindMarkdownCodeRegions(text)

	rgx := regexp.MustCompile(`(?m)!\[([^\]]*)\]\((?:<([^>\n]+)>|([^\s()]+(?:\([^\s()]*\)[^\s()]*)*))(?:[ \t]+(?:"[^"\n]*"|'[^'\n]*'))?[ \t]*\)`)

	var (
		index int
		parts []openingrouter.ChatContentPart
	)

	push := func(str, end int, suffix string) {
		if str > end {
			return
		}

		rest := text[str:end] + suffix

		total := len(parts)

		if total > 0 && parts[total-1].Type == openingrouter.ChatContentPartTypeText {
			parts[total-1].Text += rest

			return
		}

		rest = strings.TrimSpace(rest)

		if rest == "" {
			return
		}

		parts = append(parts, openingrouter.ChatContentPart{
			Type: openingrouter.ChatContentPartTypeText,
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

		if IsInsideCodeBlock(start, code) || isMarkdownEscaped(text, start) {
			push(index, end, "")

			index = end

			continue
		}

		altStart := index + location[2]
		altEnd := index + location[3]

		urlStart := index + location[4]
		urlEnd := index + location[5]

		if location[4] == -1 {
			urlStart = index + location[6]
			urlEnd = index + location[7]
		}

		alt := text[altStart:altEnd]
		url := text[urlStart:urlEnd]
		mediaType, format := classifyMediaURL(url)

		isDataUrl := strings.HasPrefix(url, "data:")
		isHttpUrl := strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://")

		if !isDataUrl && !isHttpUrl {
			push(index, end, "")

			index = end

			continue
		}

		marker := text[start:end]

		if isDataUrl {
			marker = fmt.Sprintf("![image](%s)", alt)
		} else if mediaType != "" {
			label := mediaType
			if label == "youtube" {
				label = "video"
			}

			marker = "![" + label + "](" + url + ")"
		}

		push(index, start, marker)

		switch mediaType {
		case "audio":
			if model.InputAudio {
				// ResolveAudioLinks replaces this URL with base64 before serialization.
				parts = append(parts, openingrouter.ChatContentPart{
					Type: openingrouter.ChatContentPartTypeInputAudio,
					InputAudio: &openingrouter.ChatContentInputAudio{
						Data:   url,
						Format: format,
					},
				})
			}
		case "video", "youtube":
			allowYouTube := mediaType != "youtube" || strings.HasPrefix(model.Slug, "google/")
			if model.InputVideo && allowYouTube {
				if mediaType == "youtube" {
					url = "https://www.youtube.com/watch?v=" + format
				}

				parts = append(parts, openingrouter.ChatContentPart{
					Type: openingrouter.ChatContentPartTypeVideoURL,
					VideoURL: &openingrouter.ChatContentVideoURL{
						URL: url,
					},
				})
			}
		case "image":
			if model.Vision {
				parts = append(parts, openingrouter.ChatContentPart{
					Type: openingrouter.ChatContentPartTypeImageURL,
					ImageURL: &openingrouter.ChatContentImageURL{
						Detail: openingrouter.ChatImageDetailAuto,
						URL:    url,
					},
				})
			}
		}

		index = end
	}

	return parts
}

func isMarkdownEscaped(text string, position int) bool {
	backslashes := 0

	for position > 0 && text[position-1] == '\\' {
		backslashes++
		position--
	}

	return backslashes%2 != 0
}
