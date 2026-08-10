package main

import (
	"os"
	"path/filepath"
	"strings"
)

// DownloadTracker tracks the last successfully downloaded date. The per-date
// download orchestration it used to sit beside now lives in
// internal/downloadjob (shared with the server's Studio Download screen).
type DownloadTracker struct {
	stateFile string
}

// NewDownloadTracker creates a new tracker with the given state file path
func NewDownloadTracker(stateFile string) *DownloadTracker {
	return &DownloadTracker{stateFile: stateFile}
}

// GetLastDownloadDate reads the last successful download date from state file
func (t *DownloadTracker) GetLastDownloadDate() string {
	data, err := os.ReadFile(t.stateFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// SetLastDownloadDate writes the date to the state file
func (t *DownloadTracker) SetLastDownloadDate(date string) error {
	dir := filepath.Dir(t.stateFile)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	return os.WriteFile(t.stateFile, []byte(date+"\n"), 0600)
}

// AlreadyDownloaded checks if the given date was already downloaded
func (t *DownloadTracker) AlreadyDownloaded(date string) bool {
	return t.GetLastDownloadDate() == date
}
