package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"veo3-manager/internal/types"
)

func staticToken(tok string) TokenProvider {
	return func(_ context.Context) (string, error) { return tok, nil }
}

// captured is the parsed wire envelope as a fake server sees it.
type captured struct {
	ClientContext struct {
		ProjectID       string `json:"projectId"`
		RecaptchaToken  string `json:"recaptchaToken"`
		RecaptchaAction string `json:"recaptchaAction"`
	} `json:"clientContext"`
	Requests []struct {
		TextInput struct {
			Prompt string `json:"prompt"`
		} `json:"textInput"`
		VideoModelKey   string `json:"videoModelKey"`
		AspectRatio     string `json:"aspectRatio"`
		Resolution      string `json:"resolution"`
		DurationSeconds int    `json:"durationSeconds"`
		NegativePrompt  string `json:"negativePrompt"`
		Seed            int32  `json:"seed"`
		Metadata        struct {
			SceneID string `json:"sceneId"`
		} `json:"metadata"`
	} `json:"requests"`
}

func TestClient_Submit_WireShape(t *testing.T) {
	var got captured
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/video:batchAsyncGenerateVideoText" {
			t.Errorf("wrong path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("wrong method: %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("wrong auth header: %s", got)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("envelope did not parse: %v\nbody=%s", err, body)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(SubmitResponse{
			OperationID: "op_123",
			Media:       []Media{{MediaID: "m1", Status: types.MediaStatusPending}},
		})
	}))
	defer srv.Close()

	c := New(staticToken("test-token"), WithBaseURL(srv.URL))
	resp, err := c.Submit(context.Background(), SubmitRequest{
		Prompt:      "a cat in space",
		AspectRatio: "16:9",
		Resolution:  "720p",
		DurationSec: 8,
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if resp.OperationID != "op_123" {
		t.Errorf("operationID: got %s", resp.OperationID)
	}

	// Wire shape assertions: must be wrapped in {clientContext, requests[]}
	// with the per-request fields the API expects.
	if len(got.Requests) != 1 {
		t.Fatalf("expected 1 request entry, got %d", len(got.Requests))
	}
	r := got.Requests[0]
	if r.TextInput.Prompt != "a cat in space" {
		t.Errorf("textInput.prompt: %q", r.TextInput.Prompt)
	}
	if r.VideoModelKey != DefaultModelID {
		t.Errorf("videoModelKey not defaulted: %s", r.VideoModelKey)
	}
	if r.AspectRatio != WireAspectLandscape {
		t.Errorf("aspectRatio not mapped: got %q want %q", r.AspectRatio, WireAspectLandscape)
	}
	if r.Resolution != WireResolution720p {
		t.Errorf("resolution not mapped: got %q want %q", r.Resolution, WireResolution720p)
	}
	if r.DurationSeconds != 8 {
		t.Errorf("durationSeconds: %d", r.DurationSeconds)
	}
	if r.Seed == 0 {
		t.Error("seed should be auto-generated, got 0")
	}
	if r.Metadata.SceneID == "" {
		t.Error("metadata.sceneId should be auto-generated")
	}
}

func TestClient_Submit_RecaptchaInjected(t *testing.T) {
	var hdrToken, bodyToken, bodyAction string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdrToken = r.Header.Get("X-Goog-Recaptcha-Token")
		body, _ := io.ReadAll(r.Body)
		var env captured
		_ = json.Unmarshal(body, &env)
		bodyToken = env.ClientContext.RecaptchaToken
		bodyAction = env.ClientContext.RecaptchaAction
		_ = json.NewEncoder(w).Encode(SubmitResponse{OperationID: "op_x"})
	}))
	defer srv.Close()

	provider := func(_ context.Context, action string) (string, error) {
		return "fake-recaptcha-token-" + action, nil
	}
	c := New(
		staticToken("t"),
		WithBaseURL(srv.URL),
		WithRecaptchaProvider(provider),
		WithRecaptchaAction("GENERATE_VIDEO"),
	)
	if _, err := c.Submit(context.Background(), SubmitRequest{Prompt: "x", AspectRatio: "16:9"}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if hdrToken != "fake-recaptcha-token-GENERATE_VIDEO" {
		t.Errorf("header token: %q", hdrToken)
	}
	if bodyToken != "fake-recaptcha-token-GENERATE_VIDEO" {
		t.Errorf("body token: %q", bodyToken)
	}
	if bodyAction != "GENERATE_VIDEO" {
		t.Errorf("body action: %q", bodyAction)
	}
}

func TestClient_Submit_OperationIDFromNameField(t *testing.T) {
	// Some Google LRO responses use "name" instead of "operationId".
	// Submit must fall back to that field rather than erroring "thiếu operationId".
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"name":"projects/abc/operations/op_xyz","done":false}`))
	}))
	defer srv.Close()

	c := New(staticToken("t"), WithBaseURL(srv.URL))
	resp, err := c.Submit(context.Background(), SubmitRequest{Prompt: "x", AspectRatio: "16:9"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if resp.OperationID != "projects/abc/operations/op_xyz" {
		t.Errorf("operationID fallback: got %q", resp.OperationID)
	}
}

func TestClient_Submit_OmitsEmptyMetadata(t *testing.T) {
	// Regression: prior version sent "metadata":{} on every request because
	// the struct field had ineffective omitempty. Verify the rendered body
	// does not include "metadata" when SceneID is forced empty.
	var rawBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rawBody = string(body)
		_ = json.NewEncoder(w).Encode(SubmitResponse{OperationID: "op"})
	}))
	defer srv.Close()

	c := New(staticToken("t"), WithBaseURL(srv.URL))
	// Caller-provided empty SceneID would normally be auto-filled inside
	// Submit. Pin a value via SubmitRequest.SceneID="-" then ensure non-empty
	// metadata IS present, then run a second case asserting buildSubmitEnvelope
	// directly with empty SceneID emits no metadata key.
	if _, err := c.Submit(context.Background(), SubmitRequest{Prompt: "x", AspectRatio: "16:9"}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	// Auto-filled SceneID always populates metadata, so the body MUST contain it.
	if !strings.Contains(rawBody, `"metadata"`) {
		t.Errorf("auto-filled metadata missing from body: %s", rawBody)
	}

	// Direct shape test: empty SceneID → no metadata key.
	envelope := buildSubmitEnvelope(SubmitRequest{Prompt: "x", OutputCount: 1, Seeds: []int32{1}}, "", "")
	out, _ := json.Marshal(envelope)
	if strings.Contains(string(out), `"metadata"`) {
		t.Errorf("empty metadata should be omitted, got: %s", out)
	}
}

func TestClient_Submit_MultiOutputSeedsAndScenes(t *testing.T) {
	// OutputCount > 1 must yield N requests entries each with its own seed
	// and a unique sceneId.
	envelope := buildSubmitEnvelope(SubmitRequest{
		Prompt:      "x",
		OutputCount: 3,
		Seeds:       []int32{11, 22, 33},
		SceneID:     "scene_root",
	}, "", "")
	if len(envelope.Requests) != 3 {
		t.Fatalf("expected 3 requests, got %d", len(envelope.Requests))
	}
	scenes := map[string]bool{}
	for i, r := range envelope.Requests {
		if r.Seed == 0 {
			t.Errorf("request[%d] seed unset", i)
		}
		if r.Metadata == nil || r.Metadata.SceneID == "" {
			t.Errorf("request[%d] sceneId missing", i)
			continue
		}
		if scenes[r.Metadata.SceneID] {
			t.Errorf("duplicate sceneId: %s", r.Metadata.SceneID)
		}
		scenes[r.Metadata.SceneID] = true
	}
}

func TestClient_Submit_RecaptchaProviderError(t *testing.T) {
	c := New(
		staticToken("t"),
		WithRecaptchaProvider(func(_ context.Context, _ string) (string, error) {
			return "", errors.New("captcha eval timed out")
		}),
	)
	_, err := c.Submit(context.Background(), SubmitRequest{Prompt: "x", AspectRatio: "16:9"})
	if err == nil {
		t.Fatal("expected reCAPTCHA error")
	}
	if !strings.Contains(err.Error(), "reCAPTCHA") && !strings.Contains(err.Error(), "captcha") {
		t.Errorf("error should mention captcha: %v", err)
	}
}

func TestClient_Submit_EmptyPromptRejected(t *testing.T) {
	c := New(staticToken("t"))
	_, err := c.Submit(context.Background(), SubmitRequest{Prompt: ""})
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
}

func TestClient_Submit_401ReturnsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid token"}`))
	}))
	defer srv.Close()

	c := New(staticToken("bad"), WithBaseURL(srv.URL))
	_, err := c.Submit(context.Background(), SubmitRequest{Prompt: "x", AspectRatio: "16:9"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsUnauthorized(err) {
		t.Errorf("IsUnauthorized should be true, got %v", err)
	}
	var ae *APIError
	if !errors.As(err, &ae) {
		t.Fatal("expected APIError")
	}
	if ae.StatusCode != 401 {
		t.Errorf("status code: %d", ae.StatusCode)
	}
}

func TestClient_Submit_TruncatesLargeErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(strings.Repeat("X", 5000)))
	}))
	defer srv.Close()

	c := New(staticToken("t"), WithBaseURL(srv.URL))
	_, err := c.Submit(context.Background(), SubmitRequest{Prompt: "x", AspectRatio: "16:9"})
	var ae *APIError
	if !errors.As(err, &ae) {
		t.Fatal("expected APIError")
	}
	if !strings.HasSuffix(ae.Body, "...") {
		t.Errorf("body should be truncated with ..., got len=%d", len(ae.Body))
	}
	// Bumped from 250 to 2100 — debug-friendly errors carry more diagnostic
	// info from Google's protobuf-style validation messages.
	if len(ae.Body) > 2100 {
		t.Errorf("body should be truncated to ~%d chars, got %d", errorBodyMax, len(ae.Body))
	}
	if len(ae.Body) < 1500 {
		t.Errorf("body should be at least %d chars before ellipsis, got %d", 1500, len(ae.Body))
	}
}

func TestClient_Poll_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/operations/") {
			t.Errorf("wrong path: %s", r.URL.Path)
		}
		opID := strings.TrimPrefix(r.URL.Path, "/operations/")
		_ = json.NewEncoder(w).Encode(PollResponse{
			OperationID: opID,
			Status:      types.MediaStatusInProgress,
			Media:       []Media{{MediaID: "m1", Status: types.MediaStatusInProgress}},
		})
	}))
	defer srv.Close()

	c := New(staticToken("t"), WithBaseURL(srv.URL))
	resp, err := c.Poll(context.Background(), "op_xyz")
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if resp.OperationID != "op_xyz" {
		t.Errorf("op id mismatch: %s", resp.OperationID)
	}
	if resp.Status != types.MediaStatusInProgress {
		t.Errorf("status: %v", resp.Status)
	}
}

func TestClient_WaitForCompletion_TerminalSuccess(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := hits.Add(1)
		status := types.MediaStatusInProgress
		if n >= 3 {
			status = types.MediaStatusSuccessful
		}
		_ = json.NewEncoder(w).Encode(PollResponse{
			OperationID: "op",
			Status:      status,
			Media: []Media{{
				MediaID:     "m1",
				Status:      status,
				RedirectURL: "https://example.com/redir",
			}},
		})
	}))
	defer srv.Close()

	c := New(staticToken("t"), WithBaseURL(srv.URL))
	resp, err := c.WaitForCompletion(context.Background(), "op", 50*time.Millisecond, 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForCompletion: %v", err)
	}
	if resp.Status != types.MediaStatusSuccessful {
		t.Errorf("expected success, got %v", resp.Status)
	}
	if hits.Load() < 3 {
		t.Errorf("should poll at least 3x, got %d", hits.Load())
	}
}

func TestClient_WaitForCompletion_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(PollResponse{
			OperationID: "op",
			Status:      types.MediaStatusInProgress,
		})
	}))
	defer srv.Close()

	c := New(staticToken("t"), WithBaseURL(srv.URL))
	_, err := c.WaitForCompletion(context.Background(), "op", 30*time.Millisecond, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("error should mention timeout: %v", err)
	}
}

func TestClient_WaitForCompletion_TerminalFailedReturnsResp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(PollResponse{
			OperationID: "op",
			Status:      types.MediaStatusFailed,
		})
	}))
	defer srv.Close()

	c := New(staticToken("t"), WithBaseURL(srv.URL))
	resp, err := c.WaitForCompletion(context.Background(), "op", 50*time.Millisecond, 1*time.Second)
	if err != nil {
		t.Fatalf("Failed terminal should not return error from WaitForCompletion, got %v", err)
	}
	if resp.Status != types.MediaStatusFailed {
		t.Errorf("expected failed status, got %v", resp.Status)
	}
}

func TestClient_TokenProviderError(t *testing.T) {
	c := New(func(_ context.Context) (string, error) { return "", errors.New("token fetch failed") })
	_, err := c.Submit(context.Background(), SubmitRequest{Prompt: "x", AspectRatio: "16:9"})
	if err == nil {
		t.Fatal("expected token error")
	}
	if !strings.Contains(err.Error(), "token") {
		t.Errorf("error should mention token: %v", err)
	}
}

func TestClient_EmptyTokenRejected(t *testing.T) {
	c := New(func(_ context.Context) (string, error) { return "", nil })
	_, err := c.Submit(context.Background(), SubmitRequest{Prompt: "x", AspectRatio: "16:9"})
	if err == nil {
		t.Fatal("expected empty token error")
	}
}

func TestGenerateSeeds_NUnique(t *testing.T) {
	seeds := generateSeeds(4)
	if len(seeds) != 4 {
		t.Fatalf("expected 4 seeds, got %d", len(seeds))
	}
	seen := map[int32]bool{}
	for _, s := range seeds {
		if s <= 0 {
			t.Errorf("seed must be positive, got %d", s)
		}
		if seen[s] {
			t.Errorf("seeds should be unique-ish, got duplicate %d", s)
		}
		seen[s] = true
	}
}

func TestGenerateSeeds_FitsInt32(t *testing.T) {
	// Regression test for the prior bug where seeds were 63-bit int64,
	// causing API rejection: "Invalid value at 'requests[0].seed' (TYPE_INT32)".
	seeds := generateSeeds(50)
	for _, s := range seeds {
		if s < 0 {
			t.Errorf("seed should be positive int32, got %d", s)
		}
		// Implicit bound: int32 max is 2^31-1 ≈ 2.1e9.
		if int64(s) > 0x7FFFFFFF {
			t.Errorf("seed exceeds int32 max: %d", s)
		}
	}
}

func TestMapAspectRatio(t *testing.T) {
	cases := map[string]string{
		"16:9":      WireAspectLandscape,
		"9:16":      WireAspectPortrait,
		"1:1":       "1:1", // unknown passes through
		"":          "",
	}
	for in, want := range cases {
		if got := MapAspectRatio(in); got != want {
			t.Errorf("MapAspectRatio(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMapResolution(t *testing.T) {
	cases := map[string]string{
		"720p":  WireResolution720p,
		"1080p": WireResolution1080p,
		"4k":    WireResolution4K,
		"8k":    "8k", // unknown passes through
	}
	for in, want := range cases {
		if got := MapResolution(in); got != want {
			t.Errorf("MapResolution(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseProjectIDFromURL(t *testing.T) {
	// Each entry: input URL → expected projectID. The regex matches the
	// FIRST occurrence of /project/<id> regardless of host; non-flow domains
	// match too because labs.google is the only system that calls this in
	// production (host filtering would add complexity for no benefit).
	cases := map[string]string{
		"":                                                       "",
		"https://labs.google/fx/tools/flow":                      "",
		"https://labs.google/fx/tools/flow/project/abc123":       "abc123",
		"https://labs.google/fx/tools/flow/project/abc-123_xyz":  "abc-123_xyz",
		"https://labs.google/fx/tools/flow/project/foo/something": "foo",
		"https://labs.google/fx/tools/flow/project/bar?query=1":  "bar",
		"https://labs.google/fx/tools/flow/project/baz#hash":     "baz",
		"https://labs.google/something/project/inline/inside":    "inline",
		// Non-flow host with a /project/<id> segment — regex still matches
		// the first id, which is desired behavior (host filtering is not
		// part of this helper's contract).
		"https://example.com/x/project/some-id/segment": "some-id",
	}
	for in, want := range cases {
		if got := ParseProjectIDFromURL(in); got != want {
			t.Errorf("ParseProjectIDFromURL(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestMediaStatus_Terminal(t *testing.T) {
	if !types.MediaStatusSuccessful.Terminal() {
		t.Error("Successful must be terminal")
	}
	if !types.MediaStatusFailed.Terminal() {
		t.Error("Failed must be terminal")
	}
	if types.MediaStatusPending.Terminal() {
		t.Error("Pending must not be terminal")
	}
	if types.MediaStatusInProgress.Terminal() {
		t.Error("InProgress must not be terminal")
	}
}
