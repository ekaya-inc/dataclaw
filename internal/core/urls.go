package core

import (
	"net/url"
	"strconv"
	"strings"
)

func cleanBaseURL(baseURL func() string) string {
	if baseURL == nil {
		return ""
	}
	return strings.TrimRight(strings.TrimSpace(baseURL()), "/")
}

func appendURLPath(baseURL, path string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	path = strings.TrimLeft(strings.TrimSpace(path), "/")
	if baseURL == "" {
		return "/" + path
	}
	if path == "" {
		return baseURL
	}
	return baseURL + "/" + path
}

func portFromBaseURL(baseURL string) int {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return 0
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		return 0
	}
	return port
}
