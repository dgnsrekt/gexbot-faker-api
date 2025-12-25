package staging

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dgnsrekt/gexbot-downloader/internal/api"
)

type Manager struct {
	baseDir     string
	stagingRoot string
}

func NewManager(baseDir string) *Manager {
	return &Manager{
		baseDir:     baseDir,
		stagingRoot: filepath.Join(baseDir, ".staging"),
	}
}

func (m *Manager) FinalDir() string {
	return m.baseDir
}

func (m *Manager) StagingRoot() string {
	return m.stagingRoot
}

func (m *Manager) StagingDir(date string) string {
	return filepath.Join(m.stagingRoot, date)
}

func (m *Manager) PrepareStaging(date string) error {
	dir := m.StagingDir(date)
	return os.MkdirAll(dir, 0750)
}

func (m *Manager) DownloadToStaging(ctx context.Context, client api.Client, url, destPath string) (int64, error) {
	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(destPath), 0750); err != nil {
		return 0, fmt.Errorf("creating directories: %w", err)
	}

	// Download to temp file
	tmpPath := destPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return 0, fmt.Errorf("creating temp file: %w", err)
	}

	size, err := client.DownloadFile(ctx, url, f)
	if closeErr := f.Close(); closeErr != nil && err == nil {
		err = closeErr
	}

	if err != nil {
		_ = os.Remove(tmpPath)
		return 0, fmt.Errorf("downloading file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return 0, fmt.Errorf("renaming temp file: %w", err)
	}

	return size, nil
}

func (m *Manager) CommitStaging(date string) error {
	stagingDir := m.StagingDir(date)
	finalDir := filepath.Join(m.baseDir, date)

	// Walk staging and move files
	return filepath.Walk(stagingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(stagingDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(finalDir, relPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0750); err != nil {
			return err
		}

		return os.Rename(path, destPath)
	})
}

func (m *Manager) CleanupStaging(date string) error {
	return os.RemoveAll(m.StagingDir(date))
}

// Downloader is an interface for downloading files (used for testing)
type Downloader interface {
	DownloadFile(ctx context.Context, url string, dest io.Writer) (int64, error)
}
