package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"time"

	"github.com/coalaura/openingrouter"
)

const maxLinkedAudioBytes = 32 << 20

type linkedAudio struct {
	encoded string
	size    int64
}

var (
	errLinkedAudioLimit = errors.New("linked audio exceeds the 32 MiB request limit")

	mediaIPv6Public = netip.MustParsePrefix("2000::/3")

	mediaBlockedNetworks = []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/8"),
		netip.MustParsePrefix("100.64.0.0/10"),
		netip.MustParsePrefix("192.0.0.0/24"),
		netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("198.18.0.0/15"),
		netip.MustParsePrefix("198.51.100.0/24"),
		netip.MustParsePrefix("203.0.113.0/24"),
		netip.MustParsePrefix("240.0.0.0/4"),
		netip.MustParsePrefix("2001::/32"),
		netip.MustParsePrefix("2002::/16"),
	}

	mediaHTTPClient = &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			// Resolve and validate at dial time, including redirects, to avoid DNS rebinding.
			DialContext:           dialMediaHost,
			ForceAttemptHTTP2:     true,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			IdleConnTimeout:       90 * time.Second,
		},
		CheckRedirect: checkMediaRedirect,
	}
)

func ResolveAudioLinks(ctx context.Context, request *openingrouter.ChatCompletionRequest) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	var cache map[string]linkedAudio

	remaining := int64(maxLinkedAudioBytes)

	for messageIndex := range request.Messages {
		for partIndex := range request.Messages[messageIndex].Content.Parts {
			part := &request.Messages[messageIndex].Content.Parts[partIndex]
			if part.InputAudio == nil {
				continue
			}

			rawURL := part.InputAudio.Data

			mediaType, _ := classifyMediaURL(rawURL)
			if mediaType != "audio" {
				continue
			}

			audio, ok := cache[rawURL]
			if !ok {
				data, err := fetchLinkedAudio(ctx, rawURL, remaining)
				if err != nil {
					return fmt.Errorf("load linked audio: %w", err)
				}

				audio = linkedAudio{encoded: base64.StdEncoding.EncodeToString(data), size: int64(len(data))}

				if cache == nil {
					cache = make(map[string]linkedAudio)
				}

				cache[rawURL] = audio
			}

			if audio.size > remaining {
				return errLinkedAudioLimit
			}

			remaining -= audio.size
			part.InputAudio.Data = audio.encoded
		}
	}

	return nil
}

func fetchLinkedAudio(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	if limit <= 0 {
		return nil, errLinkedAudioLimit
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	response, err := mediaHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned HTTP %d", response.StatusCode)
	}

	if response.ContentLength > limit {
		return nil, errLinkedAudioLimit
	}

	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}

	if int64(len(data)) > limit {
		return nil, errLinkedAudioLimit
	}

	if len(data) == 0 {
		return nil, errors.New("audio file is empty")
	}

	return data, nil
}

func checkMediaRedirect(request *http.Request, previous []*http.Request) error {
	if len(previous) >= 10 {
		return errors.New("too many audio redirects")
	}

	if (request.URL.Scheme != "http" && request.URL.Scheme != "https") || request.URL.User != nil {
		return errors.New("audio links must use HTTP(S) without credentials")
	}

	return nil
}

func dialMediaHost(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}

	dialer := net.Dialer{Timeout: 10 * time.Second}
	lastError := errors.New("audio links must resolve to a public IP address")

	for _, address := range addresses {
		if !isPublicMediaIP(address) {
			continue
		}

		connection, dialError := dialer.DialContext(ctx, network, net.JoinHostPort(address.String(), port))
		if dialError == nil {
			return connection, nil
		}

		lastError = dialError
	}

	return nil, lastError
}

func isPublicMediaIP(address netip.Addr) bool {
	address = address.Unmap()
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() {
		return false
	}

	if address.Is6() && !mediaIPv6Public.Contains(address) {
		return false
	}

	for _, blocked := range mediaBlockedNetworks {
		if blocked.Contains(address) {
			return false
		}
	}

	return true
}
