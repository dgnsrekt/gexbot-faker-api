## Pattern Examples: [Pattern Type]

### Pattern 1: [Descriptive Name]

**Found in**: `cmd/process.go:34-68`
**Used for**: File processing with progress tracking

```go
// File processing with progress tracking
func processFiles(cmd *cobra.Command, args []string) error {
	batchSize, _ := cmd.Flags().GetInt("batch-size")
	showProgress, _ := cmd.Flags().GetBool("progress")
	output, _ := cmd.Flags().GetString("output")

	files, err := filepath.Glob(args[0])
	if err != nil {
		return fmt.Errorf("failed to glob files: %w", err)
	}

	var bar *progressbar.ProgressBar
	if showProgress {
		bar = progressbar.Default(int64(len(files)))
	}

	results := make([]ProcessResult, 0, len(files))
	for i := 0; i < len(files); i += batchSize {
		end := i + batchSize
		if end > len(files) {
			end = len(files)
		}

		batch := files[i:end]
		batchResults := processBatch(batch)
		results = append(results, batchResults...)

		if bar != nil {
			bar.Add(len(batch))
		}
	}

	return writeResults(results, output)
}
```

**Key aspects**:

- Uses cobra flags for configuration
- Processes files in configurable batches
- Shows optional progress bar
- Handles error wrapping with context

### Pattern 2: [Alternative Approach]

**Found in**: `cmd/stream.go:45-89`
**Used for**: Streaming file processing with worker pool

```go
// Worker pool for concurrent processing
func streamProcessFiles(cmd *cobra.Command, args []string) error {
	workers, _ := cmd.Flags().GetInt("workers")
	bufferSize, _ := cmd.Flags().GetInt("buffer")

	files := make(chan string, bufferSize)
	results := make(chan ProcessResult, bufferSize)
	errors := make(chan error, 1)

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range files {
				result, err := processFile(file)
				if err != nil {
					select {
					case errors <- err:
					default:
					}
					return
				}
				results <- result
			}
		}()
	}

	// Feed files to workers
	go func() {
		defer close(files)
		for _, pattern := range args {
			filepath.Walk(pattern, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() {
					files <- path
				}
				return nil
			})
		}
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(results)
	}()

	return collectResults(results, errors)
}
```

**Key aspects**:

- Uses goroutines for concurrent processing
- Configurable worker pool size
- Buffered channels for flow control
- Proper error handling and cleanup

### Testing Patterns

**Found in**: `cmd/process_test.go:23-67`

```go
func TestProcessFiles(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		expected int
		wantErr  bool
	}{
		{
			name:     "process valid files",
			files:    []string{"test1.txt", "test2.txt"},
			expected: 2,
			wantErr:  false,
		},
		{
			name:     "handle empty input",
			files:    []string{},
			expected: 0,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test files
			testDir := setupTestFiles(t, tt.files)
			defer os.RemoveAll(testDir)

			// Create command
			cmd := &cobra.Command{}
			cmd.Flags().Int("batch-size", 10, "")
			cmd.Flags().Bool("progress", false, "")
			cmd.Flags().String("output", "", "")

			// Execute
			err := processFiles(cmd, []string{filepath.Join(testDir, "*")})

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
```

### Which Pattern to Use?

- **Batch processing**: Good for predictable workloads, progress tracking
- **Worker pool**: Better for I/O intensive tasks, variable processing times
- Both examples follow CLI best practices
- Both include proper error handling and resource cleanup

### Related Utilities

- `internal/processor/batch.go:15` - Batch processing helpers
- `internal/worker/pool.go:28` - Worker pool implementation
- `pkg/progress/bar.go:12` - Progress bar utilities
