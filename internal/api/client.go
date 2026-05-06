// Package api implements the Google Labs Veo3 generation HTTP client.
// It targets aisandbox-pa.googleapis.com/v1 with Bearer-token auth extracted
// from the Google Labs page (see internal/automation.ParseNextDataToken).
//
// Wire shape (post-2026-05 fix):
//
//	POST /v1/video:batchAsyncGenerateVideoText
//	{
//	  "clientContext": {"projectId": "...", "recaptchaToken": "...", "recaptchaAction": "submit"},
//	  "requests": [
//	    {
//	      "textInput": {"prompt": "..."},
//	      "videoModelKey": "veo_3_1_t2v_fast_ultra",
//	      "aspectRatio": "VIDEO_ASPECT_RATIO_LANDSCAPE",
//	      "resolution": "RESOLUTION_720P",
//	      "durationSeconds": 8,
//	      "seed": 1234567,
//	      "metadata": {"sceneId": "..."}
//	    }
//	  ]
//	}
//
// reCAPTCHA Enterprise token is sent BOTH in the body (clientContext.recaptchaToken)
// and via the X-Goog-Recaptcha-Token header so we cover whichever placement
// the API actually requires.
//
// Files:
//   - types.go  — request/response/media DTOs + envelope + token-provider types
//   - errors.go — APIError + helpers
//   - client.go — Client struct, options, Submit/Poll/WaitForCompletion
package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Client speaks to the Veo3 API. Construct via New().
type Client struct {
	httpClient       *http.Client
	baseURL          string
	getToken         TokenProvider
	getRecaptcha     RecaptchaProvider
	recaptchaAction  string
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

// WithRecaptchaProvider attaches a reCAPTCHA Enterprise token provider. When
// set, every Submit will fetch a fresh token via the provider and inject it
// into the request envelope + X-Goog-Recaptcha-Token header.
func WithRecaptchaProvider(p RecaptchaProvider) Option {
	return func(c *Client) { c.getRecaptcha = p }
}

// WithRecaptchaAction overrides the action string passed to the provider.
// Defaults to DefaultRecaptchaAction ("submit").
func WithRecaptchaAction(action string) Option {
	return func(c *Client) {
		if action != "" {
			c.recaptchaAction = action
		}
	}
}

// New builds a Client. The TokenProvider is mandatory.
func New(getToken TokenProvider, opts ...Option) *Client {
	c := &Client{
		httpClient:      &http.Client{Timeout: 30 * time.Second},
		baseURL:         DefaultBaseURL,
		getToken:        getToken,
		recaptchaAction: DefaultRecaptchaAction,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// do performs a request with auth. Single-pass; no automatic retry. Caller
// handles 401 → token refresh → retry if desired. extraHeaders is appended
// after the standard auth/content-type headers (use for reCAPTCHA token).
func (c *Client) do(ctx context.Context, method, path string, body any, extraHeaders map[string]string) ([]byte, error) {
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
	for k, v := range extraHeaders {
		if v != "" {
			req.Header.Set(k, v)
		}
	}

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
		excerpt := truncate(string(respBody), errorBodyMax)
		return nil, &APIError{StatusCode: resp.StatusCode, Body: excerpt, Endpoint: path}
	}
	return respBody, nil
}

// Submit posts a generation request. Builds the wire envelope, fetches a
// fresh reCAPTCHA token (if a provider is configured), and posts.
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
	if req.SceneID == "" {
		req.SceneID = randomSceneID()
	}

	// Fetch reCAPTCHA token if provider configured. Empty token from a
	// configured provider is a hard error — if reCAPTCHA is required, an
	// empty token will produce HTTP 403 anyway and the failure is more
	// confusing wrapped in an HTTP error. Fail fast here instead.
	var recaptchaToken string
	if c.getRecaptcha != nil {
		t, err := c.getRecaptcha(ctx, c.recaptchaAction)
		if err != nil {
			return nil, fmt.Errorf("lấy reCAPTCHA token: %w", err)
		}
		recaptchaToken = t
	}

	envelope := buildSubmitEnvelope(req, recaptchaToken, c.recaptchaAction)

	headers := map[string]string{}
	if recaptchaToken != "" {
		// Send token via header too — defensive, since exact placement is
		// unconfirmed (could be header-only, body-only, or both).
		headers["X-Goog-Recaptcha-Token"] = recaptchaToken
	}

	// Log the redacted envelope so the next failure is debuggable. We strip
	// the recaptchaToken from the logged copy to avoid leaking it; bearer
	// tokens are already redacted by the chromedp logger middleware.
	if logEnv, err := json.Marshal(redactEnvelope(envelope)); err == nil {
		log.Printf("[api] submit body=%s", logEnv)
	}
	log.Printf("[api] submit recaptcha=(action=%s, len=%d)", c.recaptchaAction, len(recaptchaToken))

	body, err := c.do(ctx, http.MethodPost, "/video:batchAsyncGenerateVideoText", envelope, headers)
	if err != nil {
		log.Printf("[api] submit FAILED: %v", err)
		return nil, err
	}
	log.Printf("[api] submit OK, response=%s", truncate(string(body), 500))
	var out SubmitResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse submit response: %w", err)
	}
	if out.OperationID == "" {
		// Some Google LRO responses use "name" instead of "operationId".
		// Fall back to a permissive map probe before erroring.
		var probe map[string]any
		if json.Unmarshal(body, &probe) == nil {
			if name, ok := probe["name"].(string); ok && name != "" {
				out.OperationID = name
			}
		}
	}
	if out.OperationID == "" {
		return nil, fmt.Errorf("submit response thiếu operationId; body=%s", truncate(string(body), 400))
	}
	return &out, nil
}

// redactEnvelope returns a copy of the envelope with the reCAPTCHA token
// scrubbed — for safe logging. The bearer token is set elsewhere (in the
// HTTP Authorization header) and is not part of the envelope.
func redactEnvelope(e submitEnvelope) submitEnvelope {
	if e.ClientContext.RecaptchaToken != "" {
		e.ClientContext.RecaptchaToken = fmt.Sprintf("<redacted len=%d>", len(e.ClientContext.RecaptchaToken))
	}
	return e
}

// buildSubmitEnvelope converts a high-level SubmitRequest into the wire body.
// One requestEntry is emitted per output (per seed) so each output has its
// own seed + sceneId. Aspect ratio + resolution are mapped to the wire enums.
func buildSubmitEnvelope(req SubmitRequest, recaptchaToken, recaptchaAction string) submitEnvelope {
	count := req.OutputCount
	if count <= 0 {
		count = 1
	}
	entries := make([]videoRequest, 0, count)
	for i := 0; i < count; i++ {
		var seed int32
		if i < len(req.Seeds) {
			seed = req.Seeds[i]
		} else {
			seed = randomInt31()
		}
		sceneID := req.SceneID
		if i > 0 {
			// Each output needs a unique sceneId; suffix the user-provided
			// one to keep them traceable.
			sceneID = fmt.Sprintf("%s_%d", req.SceneID, i)
		}
		var meta *videoMetadata
		if sceneID != "" {
			meta = &videoMetadata{SceneID: sceneID}
		}
		entries = append(entries, videoRequest{
			TextInput:       textInput{Prompt: req.Prompt},
			VideoModelKey:   req.ModelID,
			AspectRatio:     MapAspectRatio(req.AspectRatio),
			Resolution:      MapResolution(req.Resolution),
			DurationSeconds: req.DurationSec,
			NegativePrompt:  req.NegativePrompt,
			Seed:            seed,
			Metadata:        meta,
		})
	}
	return submitEnvelope{
		ClientContext: clientContext{
			ProjectID:       req.ProjectID,
			RecaptchaToken:  recaptchaToken,
			RecaptchaAction: recaptchaAction,
		},
		Requests: entries,
	}
}

// Poll fetches the current status of a submitted operation.
func (c *Client) Poll(ctx context.Context, operationID string) (*PollResponse, error) {
	if operationID == "" {
		return nil, errors.New("operationId rỗng")
	}
	body, err := c.do(ctx, http.MethodGet, "/operations/"+operationID, nil, nil)
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

// generateSeeds produces n random INT32 positive seeds. INT64 was previously
// used, which overflowed the API's INT32 seed field — the API rejects with
// "Invalid value at 'requests[0].seed' (TYPE_INT32)".
func generateSeeds(n int) []int32 {
	if n <= 0 {
		return nil
	}
	seeds := make([]int32, n)
	for i := 0; i < n; i++ {
		seeds[i] = randomInt31()
	}
	return seeds
}

// randomInt31 returns a positive int32 (in the range [1, 2^31-1]). Uses
// crypto/rand; falls back to time-based if rand fails.
func randomInt31() int32 {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return int32(time.Now().UnixNano() & 0x7FFFFFFF)
	}
	v := int32(binary.BigEndian.Uint32(buf) & 0x7FFFFFFF)
	if v == 0 {
		v = 1
	}
	return v
}

// randomSceneID returns a 16-hex-char id used in the per-request metadata.
// Format is opaque; any unique-per-request string should work.
func randomSceneID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("scene_%d", time.Now().UnixNano())
	}
	return "scene_" + hex.EncodeToString(buf)
}
