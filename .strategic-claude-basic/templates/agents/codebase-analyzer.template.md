## Analysis: [Feature/Component Name]

### Overview

[2-3 sentence summary of how it works]

### Entry Points

- `cmd/root.go:25` - Root command definition
- `cmd/process.go:18` - ProcessFiles() command handler

### Core Implementation

#### 1. Input Validation (`internal/validator/files.go:12-35`)

- Validates file paths and permissions using filepath.Walk
- Checks file types against allowed extensions
- Returns error if validation fails

#### 2. Data Processing (`internal/processor/file_processor.go:45-89`)

- Opens file handle at line 52
- Processes data in chunks using io.Reader at line 67
- Spawns goroutines for concurrent processing at line 78

#### 3. State Management (`internal/state/manager.go:23-67`)

- Tracks processing state using channels
- Updates progress via sync.WaitGroup
- Implements graceful shutdown on SIGTERM

### Data Flow

1. Command parsed by cobra at `cmd/process.go:18`
2. Flags validated at `internal/validator/files.go:12`
3. Processing initiated at `internal/processor/file_processor.go:45`
4. Results collected at `internal/collector/results.go:34`
5. Output written to stdout/file at `cmd/process.go:89`

### Key Patterns

- **Strategy Pattern**: ProcessorStrategy interface at `internal/processor/strategy.go:8`
- **Worker Pool**: Goroutine pool managed in `internal/worker/pool.go:15`
- **Context Cancellation**: Context passed through call chain at `cmd/process.go:25`

### Configuration

- Config loaded via viper at `internal/config/config.go:15`
- Environment variables parsed at `internal/config/config.go:28-45`
- CLI flags defined at `cmd/root.go:45-62`

### Error Handling

- Input validation errors wrapped at `internal/validator/files.go:28`
- Processing errors collected in error channel (`internal/processor/file_processor.go:82`)
- Failed operations logged via logrus to stderr
