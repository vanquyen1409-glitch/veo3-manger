package api

import (
	"context"

	"veo3-manager/internal/types"
)

// DefaultBaseURL is the AISandbox API root. Override via WithBaseURL for tests.
const DefaultBaseURL = "https://aisandbox-pa.googleapis.com/v1"

// DefaultModelID is the only Veo3 model confirmed working as of 2026.
const DefaultModelID = "veo_3_1_t2v_fast_ultra"

// SubmitRequest mirrors the JSON body posted to the submit endpoint.
type SubmitRequest struct {
	Prompt         string  `json:"prompt"`
	NegativePrompt string  `json:"negativePrompt,omitempty"`
	ModelID        string  `json:"modelId"`
	AspectRatio    string  `json:"aspectRatio"`
	Resolution     string  `json:"resolution,omitempty"`
	DurationSec    int     `json:"durationSeconds,omitempty"`
	OutputCount    int     `json:"outputCount"`
	Seeds          []int64 `json:"seeds,omitempty"`
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
