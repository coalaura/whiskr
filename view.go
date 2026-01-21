package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
)

func HandleView(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 25<<20)
	defer r.Body.Close()

	err := r.ParseMultipartForm(25 << 20)
	if err != nil {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})

		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})

		return
	}

	defer file.Close()

	buf, err := io.ReadAll(file)
	if err != nil {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})

		return
	}

	var ext string

	mime := http.DetectContentType(buf)

	switch mime {
	case "image/png":
		ext = "png"
	case "image/jpeg":
		ext = "jpg"
	case "image/webp":
		ext = "webp"
	default:
		RespondJson(w, http.StatusUnsupportedMediaType, map[string]any{
			"error": "unsupported image type",
		})

		return
	}

	sum := sha256.Sum256(buf)
	hash := hex.EncodeToString(sum[:])

	filename := fmt.Sprintf("%s%s.%s", hash[:4], hash[len(hash)-4:], ext)

	var disposition string

	if r.URL.Query().Has("download") {
		disposition = "attachment"
	} else {
		disposition = "inline"
	}

	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"", disposition, filename))
	w.Header().Set("Cache-Control", "private, max-age=0, no-store")

	w.WriteHeader(http.StatusOK)

	w.Write(buf)
}
