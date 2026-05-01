package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"veo3-manager/internal/types"
)

// ListVideos returns all stored video records sorted newest-first.
func (a *App) ListVideos() []types.Video {
	return a.store.ListVideos()
}

// DeleteVideo removes a stored video record by ID. The MP4 file on disk is
// NOT deleted (caller can use OpenPathInOS to navigate there manually).
func (a *App) DeleteVideo(id string) error {
	if err := a.store.DeleteVideo(id); err != nil {
		return err
	}
	a.emitVideosChanged()
	return nil
}

// CreateVideo validates the request, persists a "generating" row, and kicks
// off the automation pipeline in a goroutine. Returns immediately with the
// freshly-created video record. The UI listens to video:progress and
// videos:changed events for updates.
//
// The actual generation pipeline lives in app_pipeline.go.
func (a *App) CreateVideo(req types.CreateVideoRequest) (types.Video, error) {
	if req.Prompt == "" {
		return types.Video{}, fmt.Errorf("prompt không được để trống")
	}
	applyCreateRequestDefaults(&req)
	if !a.cdp.Status().Connected {
		return types.Video{}, fmt.Errorf("chưa kết nối CDP browser. Vui lòng kiểm tra kết nối ở trang Cài đặt")
	}

	video := newVideoFromRequest(req)
	if err := a.store.UpsertVideo(video); err != nil {
		return types.Video{}, err
	}
	a.emitVideosChanged()
	go a.runGeneration(video.ID)
	return video, nil
}

// applyCreateRequestDefaults fills in any zero-valued fields with the
// app-wide defaults. Mutates in place.
func applyCreateRequestDefaults(req *types.CreateVideoRequest) {
	if req.AspectRatio == "" {
		req.AspectRatio = types.AspectLandscape
	}
	if req.Resolution == "" {
		req.Resolution = types.Res720p
	}
	if req.Duration == 0 {
		req.Duration = 6
	}
}

// newVideoFromRequest builds a fresh Video record in "generating" state
// from a validated CreateVideoRequest.
func newVideoFromRequest(req types.CreateVideoRequest) types.Video {
	return types.Video{
		ID:             newVideoID(),
		Prompt:         req.Prompt,
		NegativePrompt: req.NegativePrompt,
		AspectRatio:    req.AspectRatio,
		Resolution:     req.Resolution,
		Duration:       req.Duration,
		Tags:           req.Tags,
		Status:         types.StatusGenerating,
		CreatedAt:      time.Now(),
	}
}

// newVideoID generates a 16-char hex ID for a freshly created video,
// falling back to nanosecond timestamp on entropy failure.
func newVideoID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("v_%d", time.Now().UnixNano())
	}
	return "v_" + hex.EncodeToString(b)
}
