package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type ProxyTransport struct {
	scheme string
	host   string
	token  string
}

func NewProxyTransport(host, token string) *ProxyTransport {
	scheme := "https"

	if idx := strings.Index(host, "://"); idx != -1 {
		scheme = host[:idx]
		host = host[idx+3:]
	}

	return &ProxyTransport{
		scheme: scheme,
		host:   host,
		token:  token,
	}
}

func (t *ProxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())

	clone.URL.Scheme = t.scheme
	clone.URL.Host = t.host

	clone.Host = t.host

	clone.Header.Set("X-Proxy-Auth", t.token)

	resp, err := http.DefaultTransport.RoundTrip(clone)
	if err != nil {
		return nil, translateProxyError(err)
	}

	return resp, nil
}

func ResolveProxy(name string) (*EnvProxy, error) {
	if name == "" {
		return nil, nil
	}

	for i := range env.Proxies {
		if env.Proxies[i].Name == name {
			return &env.Proxies[i], nil
		}
	}

	return nil, fmt.Errorf("unknown proxy: %q", name)
}

func ProxyNames() []string {
	names := make([]string, 0, len(env.Proxies))

	for _, proxy := range env.Proxies {
		names = append(names, proxy.Name)
	}

	return names
}

func translateProxyError(err error) error {
	if err == nil {
		return nil
	}

	var urlErr *url.Error

	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}

	msg := err.Error()

	switch {
	case strings.Contains(msg, "server gave HTTP response to HTTPS client"):
		return errors.New("proxy scheme mismatch: proxy expects HTTP, but we used HTTPS (set host to http://...)")
	case strings.Contains(msg, "server gave HTTPS response to HTTP client"):
		return errors.New("proxy scheme mismatch: proxy expects HTTPS, but we used HTTP (set host to https://...)")
	}

	return err
}
