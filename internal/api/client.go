// Package api implements the Google Labs Veo3 generation HTTP client.
// It targets aisandbox-pa.googleapis.com/v1 with Bearer-token auth extracted
// from the Google Labs page (see internal/automation.ParseNextDataToken).
//
// The exact request/response shapes here are best-effort, derived from
// public reverse-engineering work. Empirical verification against the real
// API is required; on shape mismatch, callers should see clear error
// messages including HTTP status + redacted body excerpt.
//
// Files:
//   - types.go  — request/response/media DTOs + token-provider type
//   - errors.go — APIError + helpers
//   - client.go — Client struct, options, Submit/Poll/WaitForCompletion
package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client speaks to the Veo3 API. Construct via New().
type Client struct {
	httpClient *http.Client
	baseURL    string
	getToken   TokenProvider
}

// Option mutates a Client during construction.
type Option func(*Client)

// WithHTTPClient overrides the default *http.Client (useful for tests).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithBaseURL overrides the default base URL (useful for tests).
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// New builds a Client. The TokenProvider is mandatory.
func New(getToken TokenProvider, opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    DefaultBaseURL,
		getToken:   getToken,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// do performs a request with auth. Single-pass; no automatic retry. Caller
// handles 401 → token refresh → retry if desired.
func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("lấy token: %w", err)
	}
	if token == "" {
		return nil, errors.New("token rỗng")
	}

	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(buf)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("đọc response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		excerpt := truncate(string(respBody), 200)
		return nil, &APIError{StatusCode: resp.StatusCode, Body: excerpt, Endpoint: path}
	}
	return respBody, nil
}

// Submit posts a generation request. Seeds are auto-generated if missing.
func (c *Client) Submit(ctx context.Context, req SubmitRequest) (*SubmitResponse, error) {
	if req.Prompt == "" {
		return nil, errors.New("prompt rỗng")
	}
	if req.ModelID == "" {
		req.ModelID = DefaultModelID
	}
	if req.OutputCount <= 0 {
		req.OutputCount = 1
	}
	if len(req.Seeds) == 0 {
		req.Seeds = generateSeeds(req.OutputCount)
	}
	body, err := c.do(ctx, http.MethodPost, "/video:batchAsyncGenerateVideoText", req)
	if err != nil {
		return nil, err
	}
	var out SubmitResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse submit response: %w", err)
	}
	if out.OperationID == "" {
		return nil, fmt.Errorf("submit response thiếu operationId; body=%s", truncate(string(body), 200))
	}
	return &out, nil
}

// Poll fetches the current status of a submitted operation.
func (c *Client) Poll(ctx context.Context, operationID string) (*PollResponse, error) {
	if operationID == "" {
		return nil, errors.New("operationId rỗng")
	}
	body, err := c.do(ctx, http.MethodGet, "/operations/"+operationID, nil)
	if err != nil {
		return nil, err
	}
	var out PollResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse poll response: %w", err)
	}
	if out.OperationID == "" {
		out.OperationID = operationID
	}
	return &out, nil
}

// WaitForCompletion polls every interval until the operation reaches a
// terminal status or the deadline is exceeded.
func (c *Client) WaitForCompletion(ctx context.Context, operationID string, interval, timeout time.Duration) (*PollResponse, error) {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		resp, err := c.Poll(ctx, operationID)
		if err != nil {
			return nil, err
		}
		if resp.Status.Terminal() {
			return resp, nil
		}
		if time.Now().After(deadline) {
			return resp, fmt.Errorf("hết timeout %v khi chờ video", timeout)
		}
		select {
		case <-ctx.Done():
			return resp, ctx.Err()
		case <-ticker.C:
		}
	}
}

// generateSeeds produces n random positive int64 seeds. Used when the caller
// doesn't pin specific seeds.
func generateSeeds(n int) []int64 {
	if n <= 0 {
		return nil
	}
	seeds := make([]int64, n)
	buf := make([]byte, 8)
	for i := 0; i < n; i++ {
		if _, err := rand.Read(buf); err != nil {
			seeds[i] = time.Now().UnixNano() + int64(i)
			continue
		}
		// Mask to positive 63-bit space.
		v := int64(binary.BigEndian.Uint64(buf) & 0x7FFFFFFFFFFFFFFF)
		seeds[i] = v
	}
	return seeds
}

