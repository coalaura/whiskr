package main

import (
	"fmt"
	"net/http"
)

type ProxyTransport struct {
	Inner http.RoundTripper
	Host  string
	Auth  string
}

func (t *ProxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())

	clone.URL.Scheme = "https"
	clone.URL.Host = t.Host
	clone.Host = t.Host

	clone.Header.Set("X-Whiskr-Auth", t.Auth)

	return t.Inner.RoundTrip(clone)
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
