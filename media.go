package main

import (
	"net/url"
	urlpath "path"
	"strings"
)

func classifyMediaURL(raw string) (string, string) {
	if strings.HasPrefix(raw, "data:image/") {
		return "image", ""
	}

	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Hostname() == "" || parsed.User != nil {
		return "", ""
	}

	host := strings.ToLower(parsed.Hostname())

	switch host {
	case "youtube.com", "www.youtube.com", "m.youtube.com", "music.youtube.com", "youtu.be", "www.youtu.be":
		videoID := youtubeVideoID(parsed)
		if videoID != "" {
			return "youtube", videoID
		}

		return "", ""
	}

	extension := strings.ToLower(urlpath.Ext(parsed.Path))

	switch extension {
	case ".mp3", ".wav", ".flac", ".m4a", ".ogg", ".aac", ".aiff":
		return "audio", extension[1:]
	case ".mp4", ".webm", ".mov", ".m4v", ".mpeg", ".mpg", ".avi", ".ogv", ".3gp":
		return "video", ""
	default:
		return "image", ""
	}
}

func youtubeVideoID(parsed *url.URL) string {
	var videoID string

	host := strings.ToLower(parsed.Hostname())

	switch {
	case host == "youtu.be" || host == "www.youtu.be":
		videoID = strings.TrimPrefix(parsed.Path, "/")
	case parsed.Path == "/watch":
		videoID = parsed.Query().Get("v")
	case strings.HasPrefix(parsed.Path, "/shorts/"):
		videoID = strings.TrimPrefix(parsed.Path, "/shorts/")
	case strings.HasPrefix(parsed.Path, "/embed/"):
		videoID = strings.TrimPrefix(parsed.Path, "/embed/")
	case strings.HasPrefix(parsed.Path, "/live/"):
		videoID = strings.TrimPrefix(parsed.Path, "/live/")
	}

	videoID = strings.TrimSuffix(videoID, "/")
	if !validYouTubeID(videoID) {
		return ""
	}

	return videoID
}

func validYouTubeID(videoID string) bool {
	if len(videoID) != 11 {
		return false
	}

	for _, char := range videoID {
		valid := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == '_'
		if !valid {
			return false
		}
	}

	return true
}
