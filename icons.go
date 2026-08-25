package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
)

const (
	providerIconRoute   = "/-/icon/"
	maxProviderIconSize = 2 << 20
)

var (
	providerIconMx      sync.Mutex
	providerIconNameRgx = regexp.MustCompile(`[^A-Za-z0-9_-]+`)
	providerIconPathRgx = regexp.MustCompile(`^[A-Za-z0-9_-]+\.(png|webp|jpg|jpeg|svg)$`)
)

func CacheProviderIcon(ctx context.Context, name, uri string) (string, error) {
	providerIconMx.Lock()
	defer providerIconMx.Unlock()

	source, extension, err := providerIconSource(uri)
	if err != nil {
		return "", err
	}

	filename, err := providerIconFilename(name, extension)
	if err != nil {
		return "", err
	}

	iconPath := filepath.Join(path.ProviderIcons, filename)

	info, statErr := os.Stat(iconPath)
	if statErr == nil && info.Mode().IsRegular() && info.Size() > 0 {
		return providerIconRoute + filename, nil
	}

	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return "", statErr
	}

	err = os.MkdirAll(path.ProviderIcons, 0755)
	if err != nil {
		return "", err
	}

	err = downloadProviderIcon(ctx, source, iconPath)
	if err != nil {
		return "", err
	}

	return providerIconRoute + filename, nil
}

func HandleIcon(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "path")
	if !providerIconPathRgx.MatchString(filename) {
		http.NotFound(w, r)

		return
	}

	w.Header().Set("Cache-Control", "public, max-age=3024000, immutable")

	http.ServeFile(w, r, filepath.Join(path.ProviderIcons, filename))
}

func providerIconSource(uri string) (string, string, error) {
	base := &url.URL{
		Scheme: "https",
		Host:   "openrouter.ai",
		Path:   "/",
	}

	reference, err := url.Parse(uri)
	if err != nil {
		return "", "", err
	}

	source := base.ResolveReference(reference)
	if source.Scheme != "http" && source.Scheme != "https" {
		return "", "", fmt.Errorf("unsupported icon URL scheme %q", source.Scheme)
	}

	extension := strings.ToLower(filepath.Ext(source.Path))
	if !isProviderIconExtension(extension) {
		extension = ".png"
	}

	return source.String(), extension, nil
}

func providerIconFilename(name, extension string) (string, error) {
	name = strings.Trim(providerIconNameRgx.ReplaceAllString(name, "_"), "_")
	if name == "" {
		return "", errors.New("provider name does not contain any filename-safe characters")
	}

	return strings.ToLower(name + extension), nil
}

func downloadProviderIcon(ctx context.Context, source, destination string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return err
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", source, response.Status)
	}

	contentType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(contentType, "image/") {
		return fmt.Errorf("download %s: unexpected content type %q", source, response.Header.Get("Content-Type"))
	}

	temporary, err := os.CreateTemp(filepath.Dir(destination), ".provider-icon-*")
	if err != nil {
		return err
	}

	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	written, copyErr := io.Copy(temporary, io.LimitReader(response.Body, maxProviderIconSize+1))
	closeErr := temporary.Close()

	if copyErr != nil {
		return copyErr
	}

	if closeErr != nil {
		return closeErr
	}

	if written > maxProviderIconSize {
		return fmt.Errorf("icon exceeds %d bytes", maxProviderIconSize)
	}

	err = os.Rename(temporaryPath, destination)
	if err != nil {
		return err
	}

	return nil
}

func isProviderIconExtension(extension string) bool {
	switch extension {
	case ".png", ".webp", ".jpg", ".jpeg", ".svg":
		return true
	}

	return false
}
