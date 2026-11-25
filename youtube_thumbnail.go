package main

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

const maxThumbnailBytes = 1 << 20

var thumbnailClient = &http.Client{
	Timeout: 8 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func HandleYouTubeThumbnail(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "id")
	if !validYouTubeID(videoID) {
		http.NotFound(w, r)

		return
	}

	baseURL := "https://i.ytimg.com/vi/" + videoID + "/"

	response, err := fetchThumbnail(r.Context(), baseURL+"maxresdefault.jpg")
	if err == nil && response.StatusCode == http.StatusNotFound {
		response.Body.Close()
		response, err = fetchThumbnail(r.Context(), baseURL+"hqdefault.jpg")
	}

	if err != nil {
		w.WriteHeader(http.StatusBadGateway)

		return
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK || response.ContentLength > maxThumbnailBytes {
		w.WriteHeader(http.StatusBadGateway)

		return
	}

	image, err := io.ReadAll(io.LimitReader(response.Body, maxThumbnailBytes+1))
	if err != nil || len(image) > maxThumbnailBytes || http.DetectContentType(image) != "image/jpeg" {
		w.WriteHeader(http.StatusBadGateway)

		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, max-age=86400")

	w.Write(image)
}

func fetchThumbnail(ctx context.Context, url string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	return thumbnailClient.Do(request)
}
