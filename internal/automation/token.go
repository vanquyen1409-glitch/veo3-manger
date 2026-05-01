package automation

import (
	"encoding/json"
	"errors"
	"fmt"
)

// candidatePaths lists the JSON traversal paths where Google has historically
// embedded the auth access token inside __NEXT_DATA__. We try each in order
// and return the first non-empty hit. New paths can be appended without
// breaking existing ones.
var candidatePaths = [][]string{
	{"props", "pageProps", "initialState", "auth", "accessToken"},
	{"props", "pageProps", "accessToken"},
	{"props", "pageProps", "session", "accessToken"},
	{"props", "pageProps", "session", "user", "accessToken"},
	{"props", "pageProps", "auth", "token"},
}

// ParseNextDataToken extracts the auth access token from a __NEXT_DATA__
// JSON payload. Returns an error if the JSON is malformed or none of the
// known paths produce a non-empty string.
//
// This is split out from NavigateAndExtractToken so it can be unit-tested
// without a browser.
func ParseNextDataToken(jsonText string) (string, error) {
	if jsonText == "" {
		return "", errors.New("__NEXT_DATA__ rỗng")
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(jsonText), &root); err != nil {
		return "", fmt.Errorf("parse __NEXT_DATA__ JSON: %w", err)
	}
	for _, path := range candidatePaths {
		if tok, ok := getStringPath(root, path...); ok && tok != "" {
			return tok, nil
		}
	}
	return "", errors.New("không tìm thấy access token trong __NEXT_DATA__ (chưa đăng nhập?)")
}

// getStringPath walks a nested map[string]any tree and returns the string at
// the given path, or "", false if the path doesn't resolve to a string.
func getStringPath(root map[string]any, path ...string) (string, bool) {
	var current any = root
	for _, key := range path {
		m, ok := current.(map[string]any)
		if !ok {
			return "", false
		}
		current, ok = m[key]
		if !ok {
			return "", false
		}
	}
	s, ok := current.(string)
	return s, ok
}
