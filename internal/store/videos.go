package store

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"veo3-manager/internal/types"
)

// loadVideos reads videos.json from disk into the in-memory map. If the file
// doesn't exist, writes an empty array so subsequent runs don't re-trigger
// the create-on-miss path.
func (s *Store) loadVideos() error {
	data, err := os.ReadFile(s.videosFile)
	if err != nil {
		if os.IsNotExist(err) {
			return s.saveVideosLocked()
		}
		return fmt.Errorf("read videos: %w", err)
	}
	var list []types.Video
	if err := json.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("parse videos: %w", err)
	}
	for _, v := range list {
		s.videos[v.ID] = v
	}
	return nil
}

// saveVideosLocked writes the in-memory videos map to disk as a sorted
// (newest-first) array. Caller must hold s.mu (write lock). Uses atomic
// write so a crash mid-write can't corrupt videos.json.
func (s *Store) saveVideosLocked() error {
	list := make([]types.Video, 0, len(s.videos))
	for _, v := range s.videos {
		list = append(list, v)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.After(list[j].CreatedAt) })
	return writeJSONAtomic(s.videosFile, list)
}

// ListVideos returns a snapshot of all videos sorted newest-first.
func (s *Store) ListVideos() []types.Video {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]types.Video, 0, len(s.videos))
	for _, v := range s.videos {
		list = append(list, v)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.After(list[j].CreatedAt) })
	return list
}

// GetVideo returns a video by ID. The bool reports whether the ID exists.
func (s *Store) GetVideo(id string) (types.Video, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.videos[id]
	return v, ok
}

// UpsertVideo inserts or replaces a video record and persists.
func (s *Store) UpsertVideo(v types.Video) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.videos[v.ID] = v
	return s.saveVideosLocked()
}

// DeleteVideo removes a video record by ID. The MP4 file on disk is NOT
// deleted; that is the caller's responsibility.
func (s *Store) DeleteVideo(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.videos[id]; !ok {
		return fmt.Errorf("video not found: %s", id)
	}
	delete(s.videos, id)
	return s.saveVideosLocked()
}

// UpdateStatus mutates the status (and optionally an error message) of an
// existing video. Sets CompletedAt for terminal states.
func (s *Store) UpdateStatus(id string, status types.VideoStatus, errMsg string) (types.Video, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.videos[id]
	if !ok {
		return types.Video{}, fmt.Errorf("video not found: %s", id)
	}
	v.Status = status
	v.ErrorMessage = errMsg
	if status == types.StatusCompleted || status == types.StatusFailed {
		now := time.Now()
		v.CompletedAt = &now
	}
	s.videos[id] = v
	if err := s.saveVideosLocked(); err != nil {
		return v, err
	}
	return v, nil
}
