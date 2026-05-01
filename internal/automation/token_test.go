package automation

import (
	"strings"
	"testing"
)

func TestParseNextDataToken_StandardPath(t *testing.T) {
	jsonText := `{
		"props": {
			"pageProps": {
				"initialState": {
					"auth": {
						"accessToken": "ya29.real-token-here"
					}
				}
			}
		}
	}`
	tok, err := ParseNextDataToken(jsonText)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if tok != "ya29.real-token-here" {
		t.Errorf("wrong token: %q", tok)
	}
}

func TestParseNextDataToken_AlternateShallowPath(t *testing.T) {
	jsonText := `{
		"props": {
			"pageProps": {
				"accessToken": "shallow-token"
			}
		}
	}`
	tok, err := ParseNextDataToken(jsonText)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if tok != "shallow-token" {
		t.Errorf("wrong token: %q", tok)
	}
}

func TestParseNextDataToken_SessionUserPath(t *testing.T) {
	jsonText := `{
		"props": {
			"pageProps": {
				"session": {
					"user": {
						"accessToken": "session-user-token"
					}
				}
			}
		}
	}`
	tok, err := ParseNextDataToken(jsonText)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if tok != "session-user-token" {
		t.Errorf("wrong token: %q", tok)
	}
}

func TestParseNextDataToken_FirstHitWins(t *testing.T) {
	// Both standard and shallow paths populated; standard should win because
	// it's listed first in candidatePaths.
	jsonText := `{
		"props": {
			"pageProps": {
				"accessToken": "shallow",
				"initialState": {
					"auth": {
						"accessToken": "deep"
					}
				}
			}
		}
	}`
	tok, err := ParseNextDataToken(jsonText)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if tok != "deep" {
		t.Errorf("expected first-hit (deep) to win, got %q", tok)
	}
}

func TestParseNextDataToken_NoTokenReturnsError(t *testing.T) {
	jsonText := `{
		"props": {
			"pageProps": {
				"someOtherField": "value"
			}
		}
	}`
	_, err := ParseNextDataToken(jsonText)
	if err == nil {
		t.Fatal("expected error when no token present")
	}
	if !strings.Contains(err.Error(), "không tìm thấy") {
		t.Errorf("error should mention not found, got: %v", err)
	}
}

func TestParseNextDataToken_EmptyTokenSkipped(t *testing.T) {
	// First candidate has empty string — should skip and try the next.
	jsonText := `{
		"props": {
			"pageProps": {
				"initialState": {
					"auth": {
						"accessToken": ""
					}
				},
				"accessToken": "fallback-non-empty"
			}
		}
	}`
	tok, err := ParseNextDataToken(jsonText)
	if err != nil {
		t.Fatalf("expected success when fallback non-empty, got %v", err)
	}
	if tok != "fallback-non-empty" {
		t.Errorf("should fall back to non-empty token, got %q", tok)
	}
}

func TestParseNextDataToken_MalformedJSON(t *testing.T) {
	_, err := ParseNextDataToken(`{not valid json`)
	if err == nil {
		t.Fatal("expected error on malformed JSON")
	}
}

func TestParseNextDataToken_EmptyInput(t *testing.T) {
	_, err := ParseNextDataToken("")
	if err == nil {
		t.Fatal("expected error on empty input")
	}
}

func TestParseNextDataToken_TokenIsNotString(t *testing.T) {
	// Defensive: if upstream returns a non-string at the token path, we
	// should keep walking to other candidates rather than crash.
	jsonText := `{
		"props": {
			"pageProps": {
				"initialState": {
					"auth": {
						"accessToken": 12345
					}
				},
				"accessToken": "valid-fallback"
			}
		}
	}`
	tok, err := ParseNextDataToken(jsonText)
	if err != nil {
		t.Fatalf("expected fallback to work, got %v", err)
	}
	if tok != "valid-fallback" {
		t.Errorf("wrong fallback: %q", tok)
	}
}
