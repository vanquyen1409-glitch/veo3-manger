package api

import (
	"errors"
	"fmt"
	"net/http"
)

// APIError is returned for non-2xx responses. Body is truncated to 200 chars
// so it's safe to log without leaking large payloads.
type APIError struct {
	StatusCode int
	Body       string
	Endpoint   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API %s → HTTP %d: %s", e.Endpoint, e.StatusCode, e.Body)
}

// IsUnauthorized reports whether err wraps a 401 APIError.
func IsUnauthorized(err error) bool {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae.StatusCode == http.StatusUnauthorized
	}
	return false
}

// truncate clips s to n runes and appends "..." if truncated. Used to keep
// error bodies bounded.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
