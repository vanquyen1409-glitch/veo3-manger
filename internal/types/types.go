package types

import "time"

type AspectRatio string

const (
	AspectLandscape AspectRatio = "16:9"
	AspectPortrait  AspectRatio = "9:16"
)

type Resolution string

const (
	Res720p  Resolution = "720p"
	Res1080p Resolution = "1080p"
	Res4k    Resolution = "4k"
)

type VideoStatus string

const (
	StatusGenerating VideoStatus = "generating"
	StatusCompleted  VideoStatus = "completed"
	StatusFailed     VideoStatus = "failed"
)

// MediaStatus is the per-video status returned by the Google Labs Veo3
// generation API. The string values match Google's enum exactly.
type MediaStatus string

const (
	MediaStatusPending    MediaStatus = "MEDIA_GENERATION_STATUS_PENDING"
	MediaStatusInProgress MediaStatus = "MEDIA_GENERATION_STATUS_IN_PROGRESS"
	MediaStatusSuccessful MediaStatus = "MEDIA_GENERATION_STATUS_SUCCESSFUL"
	MediaStatusFailed     MediaStatus = "MEDIA_GENERATION_STATUS_FAILED"
)

// Terminal returns true once the media is in a final state and polling
// can stop.
func (s MediaStatus) Terminal() bool {
	return s == MediaStatusSuccessful || s == MediaStatusFailed
}

type AutomationStage string

const (
	StageOpeningBrowser AutomationStage = "opening_browser"
	StageEnteringPrompt AutomationStage = "entering_prompt"
	StageWaitingVideo   AutomationStage = "waiting_video"
	StageDownloading    AutomationStage = "downloading"
	StageCompleted      AutomationStage = "completed"
	StageFailed         AutomationStage = "failed"
)

type Video struct {
	ID             string      `json:"id"`
	Prompt         string      `json:"prompt"`
	NegativePrompt string      `json:"negativePrompt"`
	AspectRatio    AspectRatio `json:"aspectRatio"`
	Resolution     Resolution  `json:"resolution"`
	Duration       int         `json:"duration"`
	Tags           []string    `json:"tags"`
	Status         VideoStatus `json:"status"`
	FilePath       string      `json:"filePath"`
	ErrorMessage   string      `json:"errorMessage"`
	CreatedAt      time.Time   `json:"createdAt"`
	CompletedAt    *time.Time  `json:"completedAt,omitempty"`
}

type CreateVideoRequest struct {
	Prompt         string      `json:"prompt"`
	NegativePrompt string      `json:"negativePrompt"`
	AspectRatio    AspectRatio `json:"aspectRatio"`
	Resolution     Resolution  `json:"resolution"`
	Duration       int         `json:"duration"`
	Tags           []string    `json:"tags"`
}

type Settings struct {
	ChromePath         string `json:"chromePath"`
	CDPPort            int    `json:"cdpPort"`
	SelectorConfigPath string `json:"selectorConfigPath"`
	OutputDir          string `json:"outputDir"`
}

type CDPStatus struct {
	Connected bool   `json:"connected"`
	Message   string `json:"message"`
}

type ProgressEvent struct {
	VideoID string          `json:"videoId"`
	Stage   AutomationStage `json:"stage"`
	Message string          `json:"message"`
}
