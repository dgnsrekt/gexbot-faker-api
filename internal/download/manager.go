package download

import (
	"context"
	"fmt"
	"os"
	"sync"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/api"
	"github.com/dgnsrekt/gexbot-downloader/internal/staging"
)

type Manager struct {
	client  api.Client
	staging *staging.Manager
	workers int
	logger  *zap.Logger
}

type BatchResult struct {
	Total    int
	Success  int
	Skipped  int
	NotFound int
	Failed   int
	Errors   []string
}

func NewManager(client api.Client, staging *staging.Manager, workers int, logger *zap.Logger) *Manager {
	return &Manager{
		client:  client,
		staging: staging,
		workers: workers,
		logger:  logger,
	}
}

func (m *Manager) Execute(ctx context.Context, tasks []Task) (*BatchResult, error) {
	result := &BatchResult{Total: len(tasks)}

	if len(tasks) == 0 {
		return result, nil
	}

	jobs := make(chan Task, len(tasks))
	results := make(chan TaskResult, len(tasks))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < m.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			m.worker(ctx, workerID, jobs, results)
		}(i)
	}

	// Send jobs
	go func() {
		for _, task := range tasks {
			select {
			case <-ctx.Done():
				return
			case jobs <- task:
			}
		}
		close(jobs)
	}()

	// Wait for workers and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	for r := range results {
		if r.Skipped {
			result.Skipped++
		} else if r.NotFound {
			result.NotFound++
		} else if r.Success {
			result.Success++
		} else {
			result.Failed++
			if r.Error != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", r.Task, r.Error))
			}
		}
	}

	return result, nil
}

func (m *Manager) worker(ctx context.Context, id int, jobs <-chan Task, results chan<- TaskResult) {
	for task := range jobs {
		select {
		case <-ctx.Done():
			return
		default:
		}

		result := m.processTask(ctx, task)

		select {
		case <-ctx.Done():
			return
		case results <- result:
		}
	}
}

func (m *Manager) processTask(ctx context.Context, task Task) TaskResult {
	result := TaskResult{Task: task}

	outputPath := task.OutputPath(m.staging.FinalDir())

	// Check if file exists (resume)
	if _, err := os.Stat(outputPath); err == nil {
		m.logger.Debug("skipping existing file", zap.String("task", task.String()))
		result.Skipped = true
		result.Success = true
		return result
	}

	m.logger.Info("downloading", zap.String("task", task.String()))

	// Get signed URL
	signedURL, err := m.client.GetDownloadURL(ctx, task.Ticker, task.Package, task.Category, task.Date)
	if err != nil {
		if err == api.ErrNotFound {
			m.logger.Debug("not found", zap.String("task", task.String()))
			result.NotFound = true
			return result
		}
		result.Error = err
		return result
	}

	// Download to staging
	stagingPath := task.OutputPath(m.staging.StagingRoot())
	size, err := m.staging.DownloadToStaging(ctx, m.client, signedURL, stagingPath)
	if err != nil {
		result.Error = err
		return result
	}

	result.Success = true
	result.BytesSize = size
	m.logger.Info("downloaded", zap.String("task", task.String()), zap.Int64("bytes", size))

	return result
}
