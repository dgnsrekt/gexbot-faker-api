package staging

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type mockClient struct {
	data []byte
}

func (m *mockClient) GetDownloadURL(ctx context.Context, ticker, pkg, category, date string) (string, error) {
	return "https://example.com/file.json", nil
}

func (m *mockClient) DownloadFile(ctx context.Context, url string, dest io.Writer) (int64, error) {
	n, err := dest.Write(m.data)
	return int64(n), err
}

func TestStagingManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "staging-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	mgr := NewManager(tmpDir)

	// Test FinalDir
	if mgr.FinalDir() != tmpDir {
		t.Errorf("expected FinalDir %s, got %s", tmpDir, mgr.FinalDir())
	}

	// Test StagingDir
	expectedStaging := filepath.Join(tmpDir, ".staging", "2025-11-14")
	if mgr.StagingDir("2025-11-14") != expectedStaging {
		t.Errorf("expected StagingDir %s, got %s", expectedStaging, mgr.StagingDir("2025-11-14"))
	}

	// Test PrepareStaging
	if err := mgr.PrepareStaging("2025-11-14"); err != nil {
		t.Fatalf("PrepareStaging failed: %v", err)
	}

	if _, err := os.Stat(expectedStaging); os.IsNotExist(err) {
		t.Error("staging directory not created")
	}

	// Test DownloadToStaging
	client := &mockClient{data: []byte(`{"test": "data"}`)}
	destPath := filepath.Join(mgr.StagingDir("2025-11-14"), "SPX", "state", "gex_full.json")

	size, err := mgr.DownloadToStaging(context.Background(), client, "https://example.com/file.json", destPath)
	if err != nil {
		t.Fatalf("DownloadToStaging failed: %v", err)
	}

	if size != int64(len(client.data)) {
		t.Errorf("expected size %d, got %d", len(client.data), size)
	}

	// Verify file exists and content matches
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}

	if string(content) != string(client.data) {
		t.Errorf("content mismatch: expected %s, got %s", string(client.data), string(content))
	}

	// Verify no .tmp file exists
	if _, err := os.Stat(destPath + ".tmp"); !os.IsNotExist(err) {
		t.Error("temp file should not exist after successful download")
	}

	// Test CommitStaging
	if err := mgr.CommitStaging("2025-11-14"); err != nil {
		t.Fatalf("CommitStaging failed: %v", err)
	}

	finalPath := filepath.Join(tmpDir, "2025-11-14", "SPX", "state", "gex_full.json")
	if _, err := os.Stat(finalPath); os.IsNotExist(err) {
		t.Error("file not moved to final directory")
	}

	// Test CleanupStaging
	if err := mgr.CleanupStaging("2025-11-14"); err != nil {
		t.Fatalf("CleanupStaging failed: %v", err)
	}

	if _, err := os.Stat(mgr.StagingDir("2025-11-14")); !os.IsNotExist(err) {
		t.Error("staging directory should be removed after cleanup")
	}
}
