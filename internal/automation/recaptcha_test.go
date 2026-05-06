package automation

import (
	"encoding/json"
	"testing"
)

func TestPickBestCapture_Empty(t *testing.T) {
	got := PickBestCapture(nil, "")
	if got.Action != "" || got.SiteKey != "" {
		t.Errorf("expected zero value for empty input, got %+v", got)
	}
}

func TestPickBestCapture_LatestNonEmpty(t *testing.T) {
	calls := []CapturedRecaptchaCall{
		{SiteKey: "6L_old", Action: "OLD_ACTION", T: 1},
		{SiteKey: "6L_new", Action: "NEW_ACTION", T: 2},
	}
	got := PickBestCapture(calls, "")
	if got.Action != "NEW_ACTION" {
		t.Errorf("expected latest action, got %q", got.Action)
	}
	if got.SiteKey != "6L_new" {
		t.Errorf("expected latest siteKey, got %q", got.SiteKey)
	}
}

func TestPickBestCapture_PrefersMatchingSiteKey(t *testing.T) {
	// When preferredSiteKey is set, prefer the latest entry whose SiteKey
	// matches, not just the latest one. Avoids grabbing actions from
	// unrelated reCAPTCHA widgets on the page (e.g. login form).
	calls := []CapturedRecaptchaCall{
		{SiteKey: "6L_video", Action: "GENERATE_VIDEO", T: 1},
		{SiteKey: "6L_login", Action: "LOGIN", T: 2},
	}
	got := PickBestCapture(calls, "6L_video")
	if got.Action != "GENERATE_VIDEO" {
		t.Errorf("expected matching siteKey to win, got %+v", got)
	}
}

func TestPickBestCapture_FallbackWhenNoMatch(t *testing.T) {
	// preferredSiteKey doesn't match any entry — return the latest non-empty
	// action anyway (better than nothing).
	calls := []CapturedRecaptchaCall{
		{SiteKey: "6L_a", Action: "ACT_A", T: 1},
		{SiteKey: "6L_b", Action: "ACT_B", T: 2},
	}
	got := PickBestCapture(calls, "6L_zzz")
	if got.Action != "ACT_B" {
		t.Errorf("expected latest fallback, got %+v", got)
	}
}

func TestPickBestCapture_SkipsEmptyAction(t *testing.T) {
	// Entries with empty Action are useless — skip them. The latest
	// non-empty action wins.
	calls := []CapturedRecaptchaCall{
		{SiteKey: "6L_x", Action: "REAL", T: 1},
		{SiteKey: "6L_x", Action: "", T: 2},
	}
	got := PickBestCapture(calls, "")
	if got.Action != "REAL" {
		t.Errorf("expected to skip empty action, got %q", got.Action)
	}
}

func TestCapturedRecaptchaCall_JSONRoundtrip(t *testing.T) {
	// The drainCaptureJS returns a JSON array shape like:
	//   [{"siteKey":"6L_abc","action":"GENERATE_VIDEO","t":1700000000000}]
	// Verify our struct unmarshals it cleanly.
	raw := `[{"siteKey":"6L_abc","action":"GENERATE_VIDEO","t":1700000000000}]`
	var got []CapturedRecaptchaCall
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if got[0].SiteKey != "6L_abc" || got[0].Action != "GENERATE_VIDEO" || got[0].T != 1700000000000 {
		t.Errorf("decoded mismatch: %+v", got[0])
	}
}

func TestTruncateForLog(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"short", 100, "short"},
		{"abcdefghij", 5, "abcde..."},
		{"", 5, ""},
	}
	for _, c := range cases {
		got := truncateForLog(c.in, c.n)
		if got != c.want {
			t.Errorf("truncateForLog(%q, %d) = %q; want %q", c.in, c.n, got, c.want)
		}
	}
}
