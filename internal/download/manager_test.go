package download

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/api"
	"github.com/dgnsrekt/gexbot-downloader/internal/staging"
)

type mockClient struct {
	urls     map[string]string
	data     []byte
	notFound []string
}

func (m *mockClient) GetDownloadURL(ctx context.Context, ticker, pkg, category, date string) (string, error) {
	key := ticker + "/" + pkg + "/" + category + "/" + date
	for _, nf := range m.notFound {
		if nf == key {
			return "", api.ErrNotFound
		}
	}
	if url, ok := m.urls[key]; ok {
		return url, nil
	}
	return "https://example.com/file.json", nil
}

func (m *mockClient) DownloadFile(ctx context.Context, url string, dest io.Writer) (int64, error) {
	n, err := dest.Write(m.data)
	return int64(n), err
}

func TestDownloadManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "download-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	client := &mockClient{
		data:     []byte(`{"test": "data"}`),
		notFound: []string{"SPX/state/gex_one/2025-11-14"},
	}

	stgMgr := staging.NewManager(tmpDir)
	logger, _ := zap.NewDevelopment()
	mgr := NewManager(client, stgMgr, 2, logger)

	tasks := []Task{
		{Ticker: "SPX", Package: "state", Category: "gex_full", Date: "2025-11-14"},
		{Ticker: "SPX", Package: "state", Category: "gex_zero", Date: "2025-11-14"},
		{Ticker: "SPX", Package: "state", Category: "gex_one", Date: "2025-11-14"}, // This one is not found
	}

	result, err := mgr.Execute(context.Background(), tasks)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Total != 3 {
		t.Errorf("expected total 3, got %d", result.Total)
	}

	if result.Success != 2 {
		t.Errorf("expected 2 successful, got %d", result.Success)
	}

	if result.NotFound != 1 {
		t.Errorf("expected 1 not found, got %d", result.NotFound)
	}

	// Verify files were created in staging (path includes date/ticker/package/category.json within staging)
	stagingPath := filepath.Join(tmpDir, ".staging", "2025-11-14", "SPX", "state", "gex_full.json")
	if _, err := os.Stat(stagingPath); os.IsNotExist(err) {
		t.Errorf("expected file in staging directory at %s", stagingPath)
	}
}

func TestDownloadManager_Resume(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "download-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	client := &mockClient{
		data: []byte(`{"test": "data"}`),
	}

	stgMgr := staging.NewManager(tmpDir)
	logger, _ := zap.NewDevelopment()
	mgr := NewManager(client, stgMgr, 1, logger)

	// Pre-create a file in the final directory
	finalPath := filepath.Join(tmpDir, "2025-11-14", "SPX", "state", "gex_full.json")
	if err := os.MkdirAll(filepath.Dir(finalPath), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(finalPath, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}

	tasks := []Task{
		{Ticker: "SPX", Package: "state", Category: "gex_full", Date: "2025-11-14"},
	}

	result, err := mgr.Execute(context.Background(), tasks)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", result.Skipped)
	}

	// Verify original file wasn't modified
	content, _ := os.ReadFile(finalPath)
	if string(content) != "existing" {
		t.Error("existing file was modified")
	}
}

func TestTask(t *testing.T) {
	task := Task{
		Ticker:   "SPX",
		Package:  "state",
		Category: "gex_full",
		Date:     "2025-11-14",
	}

	if task.APIPath() != "SPX/state/gex_full/2025-11-14" {
		t.Errorf("unexpected APIPath: %s", task.APIPath())
	}

	expectedOutput := filepath.Join("data", "2025-11-14", "SPX", "state", "gex_full.json")
	if task.OutputPath("data") != expectedOutput {
		t.Errorf("expected OutputPath %s, got %s", expectedOutput, task.OutputPath("data"))
	}

	if task.String() != "2025-11-14/SPX/state/gex_full" {
		t.Errorf("unexpected String: %s", task.String())
	}
}
