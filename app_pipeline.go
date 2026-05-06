package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"veo3-manager/internal/api"
	"veo3-manager/internal/automation"
	"veo3-manager/internal/types"
)

// pollInterval is how often we ask the Veo3 API for status. pollTimeout is
// the upper bound before we give up on a generation operation. dlTimeout
// caps the time a single MP4 download is allowed to take.
const (
	pollInterval = 10 * time.Second
	pollTimeout  = 5 * time.Minute
	dlTimeout    = 90 * time.Second
)

// runGeneration executes the full video generation pipeline:
//   - Stage 1: connect to running Chrome via CDP
//   - Stage 2: navigate to labs.google + extract auth token from __NEXT_DATA__
//   - Stage 3: submit prompt to Veo3 API + poll until terminal status
//   - Stage 4: navigate to redirectURL, capture signed URL via Network event,
//     download MP4 to OutputDir
//   - Stage 5: update store with file path + completed status
//
// API request/response shapes (Stage 3) are best-effort guesses against the
// real Google Labs API and may need adjustment. On shape mismatch, errors
// surface to the UI with HTTP status + truncated body excerpt.
func (a *App) runGeneration(videoID string) {
	browser, err := a.connectBrowser(videoID)
	if err != nil {
		return
	}
	defer browser.Close()

	token, err := a.extractToken(videoID, browser)
	if err != nil {
		return
	}

	redirectURL, err := a.submitAndWait(videoID, token, browser)
	if err != nil {
		return
	}

	settings := a.store.GetSettings()
	destPath := filepath.Join(settings.OutputDir, videoID+".mp4")
	if err := a.downloadVideo(videoID, browser, redirectURL, destPath); err != nil {
		return
	}

	a.finalizeGenerated(videoID, destPath)
}

func (a *App) connectBrowser(videoID string) (*automation.Browser, error) {
	a.emitStage(videoID, types.StageOpeningBrowser, "Đang kết nối CDP...")
	settings := a.store.GetSettings()
	browser, err := automation.Connect(a.ctx, settings.CDPPort)
	if err != nil {
		a.failVideo(videoID, fmt.Errorf("kết nối CDP: %w", err))
		return nil, err
	}
	return browser, nil
}

func (a *App) extractToken(videoID string, browser *automation.Browser) (string, error) {
	a.emitStage(videoID, types.StageEnteringPrompt, "Đang mở Google Labs và lấy token...")
	token, err := browser.NavigateAndExtractToken(automation.DefaultFlowURL)
	if err != nil {
		a.failVideo(videoID, fmt.Errorf("trích xuất token: %w", err))
		return "", err
	}
	if len(token) < 10 {
		err := fmt.Errorf("token nhận được không hợp lệ")
		a.failVideo(videoID, err)
		return "", err
	}
	return token, nil
}

// submitAndWait posts the prompt and polls until terminal status. Returns
// the redirect URL of the first successful media on success.
func (a *App) submitAndWait(videoID, token string, browser *automation.Browser) (string, error) {
	a.emitStage(videoID, types.StageWaitingVideo, "Đang gửi prompt và chờ video...")
	video, ok := a.store.GetVideo(videoID)
	if !ok {
		err := fmt.Errorf("video biến mất khỏi store: %s", videoID)
		a.failVideo(videoID, err)
		return "", err
	}

	// Detect the reCAPTCHA Enterprise site key on the labs.google page once,
	// then build a token provider that fetches a fresh token per submit.
	// If detection fails (no key in DOM), proceed without reCAPTCHA — the
	// API will return HTTP 403 with a clear error if the token is required.
	siteKey, _ := browser.DetectRecaptchaSiteKey(a.ctx)

	// Drain any captured grecaptcha.enterprise.execute calls the page made
	// (e.g. preflight risk evaluations triggered on page load). The captured
	// action is the EXACT string the page uses; using it instead of our
	// hardcoded "submit" default is what fixes HTTP 403 reCAPTCHA evaluation
	// failures. Fallback chain (in priority order):
	//   1. Latest fresh capture from this drain
	//   2. Browser's last cached observation (from a prior submit in this session)
	//   3. Hardcoded DefaultRecaptchaAction ("submit") — original behavior
	recaptchaAction := api.DefaultRecaptchaAction
	captureSource := "fallback"
	if calls, err := browser.DrainCapturedRecaptcha(a.ctx); err != nil {
		log.Printf("[pipeline] drain capture failed (sẽ dùng default): %v", err)
	} else if best := automation.PickBestCapture(calls, siteKey); best.Action != "" {
		recaptchaAction = best.Action
		captureSource = "captured"
		if best.SiteKey != "" {
			siteKey = best.SiteKey
		}
	} else if cached := browser.LastRecaptcha(); cached.Action != "" {
		// Fresh drain empty but we already learned the action in a prior
		// submit — reuse it instead of regressing to the default.
		recaptchaAction = cached.Action
		captureSource = "cached"
		if cached.SiteKey != "" {
			siteKey = cached.SiteKey
		}
	}
	log.Printf("[pipeline] reCAPTCHA siteKey=%s action=%q source=%s", siteKey, recaptchaAction, captureSource)

	clientOpts := []api.Option{api.WithRecaptchaAction(recaptchaAction)}
	if siteKey != "" {
		clientOpts = append(clientOpts, api.WithRecaptchaProvider(
			func(ctx context.Context, action string) (string, error) {
				return browser.GetRecaptchaEnterpriseToken(ctx, siteKey, action)
			},
		))
	}

	// Project ID from current URL (e.g. /project/<id>). Optional — kept
	// behind a log so a wrong/missing extraction is visible without breaking
	// the submit (the field is omitempty in the envelope).
	projectID := api.ParseProjectIDFromURL(browser.CurrentURL(a.ctx))
	if projectID != "" {
		log.Printf("[pipeline] projectId=%s", projectID)
	}

	client := api.New(func(_ context.Context) (string, error) { return token, nil }, clientOpts...)
	submitResp, err := client.Submit(a.ctx, api.SubmitRequest{
		Prompt:         video.Prompt,
		NegativePrompt: video.NegativePrompt,
		AspectRatio:    string(video.AspectRatio),
		Resolution:     string(video.Resolution),
		DurationSec:    video.Duration,
		OutputCount:    1,
		ProjectID:      projectID,
	})
	if err != nil {
		a.failVideo(videoID, fmt.Errorf("submit prompt: %w", err))
		return "", err
	}
	pollResp, err := client.WaitForCompletion(a.ctx, submitResp.OperationID, pollInterval, pollTimeout)
	if err != nil {
		a.failVideo(videoID, fmt.Errorf("chờ video: %w", err))
		return "", err
	}
	if pollResp.Status != types.MediaStatusSuccessful {
		err := fmt.Errorf("status không thành công (%s): %s", pollResp.Status, pickMediaError(pollResp.Media))
		a.failVideo(videoID, err)
		return "", err
	}
	if len(pollResp.Media) == 0 || pollResp.Media[0].RedirectURL == "" {
		err := fmt.Errorf("response không có redirectURL")
		a.failVideo(videoID, err)
		return "", err
	}
	return pollResp.Media[0].RedirectURL, nil
}

func (a *App) downloadVideo(videoID string, browser *automation.Browser, redirectURL, destPath string) error {
	a.emitStage(videoID, types.StageDownloading, "Đang tải video về máy...")
	if err := browser.DownloadVideo(a.ctx, redirectURL, destPath, dlTimeout); err != nil {
		a.failVideo(videoID, fmt.Errorf("tải video: %w", err))
		return err
	}
	return nil
}

func (a *App) finalizeGenerated(videoID, destPath string) {
	finalVideo, ok := a.store.GetVideo(videoID)
	if !ok {
		a.failVideo(videoID, fmt.Errorf("video biến mất sau khi tải"))
		return
	}
	finalVideo.FilePath = destPath
	finalVideo.Status = types.StatusCompleted
	now := time.Now()
	finalVideo.CompletedAt = &now
	if err := a.store.UpsertVideo(finalVideo); err != nil {
		a.failVideo(videoID, err)
		return
	}
	a.emitStage(videoID, types.StageCompleted, "Hoàn thành — video đã lưu vào "+destPath)
	a.emitVideosChanged()
}

// failVideo updates the store status to failed, emits a progress event with
// the error, and pushes a videos:changed notification so the UI refreshes.
// Used as the single error-exit path inside runGeneration helpers.
func (a *App) failVideo(videoID string, err error) {
	msg := err.Error()
	if _, dbErr := a.store.UpdateStatus(videoID, types.StatusFailed, msg); dbErr != nil {
		msg = fmt.Sprintf("%s (kèm lỗi DB: %v)", msg, dbErr)
	}
	a.emitStage(videoID, types.StageFailed, msg)
	a.emitVideosChanged()
}

// emitStage is a thin wrapper around emitProgress for the common case of
// "advance the pipeline to a named stage with this human-readable message".
func (a *App) emitStage(videoID string, stage types.AutomationStage, message string) {
	a.emitProgress(types.ProgressEvent{
		VideoID: videoID,
		Stage:   stage,
		Message: message,
	})
}

// pickMediaError returns the first media's error message, or a generic
// fallback when the response carries no per-media error detail.
func pickMediaError(media []api.Media) string {
	if len(media) > 0 && media[0].Error != "" {
		return media[0].Error
	}
	return "video bị từ chối hoặc lỗi"
}
