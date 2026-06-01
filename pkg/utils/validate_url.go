package utils

import (
	"net/url"
)

// ValidateURL checks if a URL has a valid format (scheme and host).
// It does not check for reachability.
func ValidateURL(urlToTest string) bool {
	// Simple format check using Go's standard library.
	// This is sufficient for a URL shortener as we don't need to
	// guarantee reachability, only that the URL is well-formed.
	parsedURL, err := url.ParseRequestURI(urlToTest)
	if err != nil {
		return false
	}

	// Ensure the URL has a scheme (http, https) and a host.
	return parsedURL.Scheme != "" && parsedURL.Host != ""
}
