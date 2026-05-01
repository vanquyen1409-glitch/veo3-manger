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

func TestClient_Submit_Success(t *testing.T) {
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
		var sub SubmitRequest
		if err := json.Unmarshal(body, &sub); err != nil {
			t.Fatalf("bad request body: %v", err)
		}
		if sub.Prompt != "a cat in space" {
			t.Errorf("prompt not forwarded: %q", sub.Prompt)
		}
		if sub.ModelID != DefaultModelID {
			t.Errorf("model default not applied: %s", sub.ModelID)
		}
		if sub.OutputCount != 1 {
			t.Errorf("outputCount default not applied: %d", sub.OutputCount)
		}
		if len(sub.Seeds) == 0 {
			t.Error("seeds should be auto-generated")
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
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if resp.OperationID != "op_123" {
		t.Errorf("operationID: got %s", resp.OperationID)
	}
	if len(resp.Media) != 1 || resp.Media[0].MediaID != "m1" {
		t.Errorf("media not parsed: %+v", resp.Media)
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	if len(ae.Body) > 250 {
		t.Errorf("body should be truncated to ~200 chars, got %d", len(ae.Body))
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
	seen := map[int64]bool{}
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
