package api

import (
	"context"
	"regexp"

	"veo3-manager/internal/types"
)

// DefaultBaseURL is the AISandbox API root. Override via WithBaseURL for tests.
const DefaultBaseURL = "https://aisandbox-pa.googleapis.com/v1"

// DefaultModelID is the only Veo3 model confirmed working as of 2026.
const DefaultModelID = "veo_3_1_t2v_fast_ultra"

// DefaultRecaptchaAction is the action string used for reCAPTCHA Enterprise
// challenges on labs.google. Empirical capture via cmd/uidrive may show a
// different value (e.g. "GENERATE_VIDEO" or "VEO3_SUBMIT") — override via the
// Client option if needed.
const DefaultRecaptchaAction = "submit"

// Aspect-ratio + resolution enum string values used in the wire envelope.
// Values follow the Vertex AI / Veo public naming. Easy to swap if labs.google
// uses different constants — change in one place.
const (
	WireAspectLandscape = "VIDEO_ASPECT_RATIO_LANDSCAPE"
	WireAspectPortrait  = "VIDEO_ASPECT_RATIO_PORTRAIT"

	WireResolution720p  = "RESOLUTION_720P"
	WireResolution1080p = "RESOLUTION_1080P"
	WireResolution4K    = "RESOLUTION_4K"
)

// SubmitRequest is the high-level DTO the app passes to Client.Submit. The
// wire envelope is built inside Submit; callers don't see it.
type SubmitRequest struct {
	Prompt         string
	NegativePrompt string
	ModelID        string
	AspectRatio    string
	Resolution     string
	DurationSec    int
	OutputCount    int
	Seeds          []int32 // optional; auto-generated when empty
	SceneID        string  // optional; auto-generated UUID-ish when empty
	ProjectID      string  // optional; passed through clientContext.projectId
}

// submitEnvelope is the wire body sent to /v1/video:batchAsyncGenerateVideoText.
// Shape derived from Keysight HAR analysis + observed API errors:
//   - top-level wrapper has clientContext + requests
//   - each request entry produces one output, has its own seed
//   - prompt is nested under textInput
//   - seed is INT32 singular (not the prior []int64 slice)
//
// recaptchaToken is duplicated into clientContext for servers that read it
// from the body; the same token is also sent via header in client.do.
type submitEnvelope struct {
	ClientContext clientContext  `json:"clientContext"`
	Requests      []videoRequest `json:"requests"`
}

type clientContext struct {
	ProjectID       string `json:"projectId,omitempty"`
	RecaptchaToken  string `json:"recaptchaToken,omitempty"`
	RecaptchaAction string `json:"recaptchaAction,omitempty"`
}

type videoRequest struct {
	TextInput       textInput      `json:"textInput"`
	VideoModelKey   string         `json:"videoModelKey"`
	AspectRatio     string         `json:"aspectRatio,omitempty"`
	Resolution      string         `json:"resolution,omitempty"`
	DurationSeconds int            `json:"durationSeconds,omitempty"`
	NegativePrompt  string         `json:"negativePrompt,omitempty"`
	Seed            int32          `json:"seed"`
	// Pointer + omitempty so an empty videoMetadata{} is omitted entirely
	// rather than serialized as {"metadata":{}}, which the API may reject.
	Metadata *videoMetadata `json:"metadata,omitempty"`
}

type textInput struct {
	Prompt string `json:"prompt"`
}

type videoMetadata struct {
	SceneID string `json:"sceneId,omitempty"`
}

// Media is one generated clip from the API. Fields are populated as the job
// progresses through the polling lifecycle.
type Media struct {
	MediaID     string            `json:"mediaId"`
	Status      types.MediaStatus `json:"status"`
	RedirectURL string            `json:"redirectUrl"`
	Seed        int64             `json:"seed"`
	Error       string            `json:"error,omitempty"`
}

// SubmitResponse is the response to a successful submit. The operation ID
// is opaque and used for subsequent polling.
type SubmitResponse struct {
	OperationID string  `json:"operationId"`
	Media       []Media `json:"media"`
}

// PollResponse is the response to a poll request.
type PollResponse struct {
	OperationID string            `json:"operationId"`
	Status      types.MediaStatus `json:"status"`
	Media       []Media           `json:"media"`
}

// TokenProvider returns a fresh Bearer token. The Client calls this once per
// request; on 401 it calls again to allow caller to re-extract.
type TokenProvider func(ctx context.Context) (string, error)

// RecaptchaProvider returns a fresh reCAPTCHA Enterprise token tied to the
// given action. Called once per Submit. Return ("", nil) to skip reCAPTCHA
// (e.g. for tests against a fake server).
type RecaptchaProvider func(ctx context.Context, action string) (string, error)

// MapAspectRatio converts an app-side AspectRatio string ("16:9"/"9:16")
// into the API wire enum. Unknown values pass through unchanged so callers
// can override.
func MapAspectRatio(s string) string {
	switch s {
	case "16:9":
		return WireAspectLandscape
	case "9:16":
		return WireAspectPortrait
	}
	return s
}

// MapResolution converts an app-side Resolution ("720p"/"1080p"/"4k") into
// the API wire enum. Unknown values pass through unchanged.
func MapResolution(s string) string {
	switch s {
	case "720p":
		return WireResolution720p
	case "1080p":
		return WireResolution1080p
	case "4k":
		return WireResolution4K
	}
	return s
}

// projectIDPattern matches the "/project/<id>" segment in labs.google Flow
// URLs (e.g. https://labs.google/fx/tools/flow/project/abc123def456). The id
// is opaque (alnum + dashes/underscores).
var projectIDPattern = regexp.MustCompile(`/project/([A-Za-z0-9_\-]+)`)

// ParseProjectIDFromURL extracts the project ID from a Google Labs Flow URL.
// Returns "" if the URL has no /project/<id> segment (e.g. the user is on
// the tool root before opening any project).
//
// The aisandbox API does not strictly require projectId for video submit —
// the bearer token + reCAPTCHA cover auth/risk — but populating it matches
// what the official UI sends and avoids drifting away from the wire shape
// observed in cmd/uidrive captures.
func ParseProjectIDFromURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	m := projectIDPattern.FindStringSubmatch(rawURL)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}
