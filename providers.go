package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
)

const ProviderIconCache = "providers"

var (
	providerNameRgx = regexp.MustCompile(`[^\w-]+`)
	providerIconRgx = regexp.MustCompile(`(?m)^[\w-]+\.[a-z]+$`)
)

func LoadProviderIcon(name, icon string) string {
	icon, ext := CleanProviderIconUrl(icon)
	if icon == "" {
		return ""
	}

	name = CleanProviderFilename(name, ext)
	path := filepath.Join(ProviderIconCache, name)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if _, err := os.Stat(ProviderIconCache); os.IsNotExist(err) {
			os.MkdirAll(ProviderIconCache, 0755)
		}

		err := DownloadProviderIcon(icon, path)
		if err != nil {
			log.Warnf("Failed to load icon %q: %v\n", icon, err)

			return ""
		}
	}

	return fmt.Sprintf("/-/provider/%s", name)
}

func DownloadProviderIcon(url, path string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New(resp.Status)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

func HandleProviderIcon(w http.ResponseWriter, r *http.Request) {
	icon := chi.URLParam(r, "icon")

	if !providerIconRgx.MatchString(icon) {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	http.ServeFile(w, r, filepath.Join(ProviderIconCache, icon))
}

func CleanProviderIconUrl(icon string) (string, string) {
	if icon == "" {
		return "", ""
	}

	u, err := url.Parse(icon)
	if err != nil {
		return "", ""
	}

	// relative paths
	if !u.IsAbs() && strings.HasPrefix(icon, "/") {
		u.Scheme = "https"
		u.Host = "openrouter.ai"

		return u.String(), filepath.Ext(u.Path)
	}

	// extract embedded github avatar
	if u.Host == "t0.gstatic.com" && u.Path == "/faviconV2" {
		embeddedURL := u.Query().Get("url")

		if eu, err := url.Parse(embeddedURL); err == nil && eu.Host == "avatars.githubusercontent.com" {
			return embeddedURL, filepath.Ext(eu.Path)
		}
	}

	return icon, filepath.Ext(u.Path)
}

func CleanProviderFilename(name, ext string) string {
	name = providerNameRgx.ReplaceAllString(name, "_")

	if ext == "" {
		ext = ".png"
	}

	return fmt.Sprintf("%s%s", name, ext)
}
