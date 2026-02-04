# PRD: DRY Refactor Pass 1 (WebSocket streamers + shared helpers)

## Goal
Reduce the biggest DRY violations in `gexbot-faker-api` without changing externally observable behavior.

Primary focus: WebSocket streamers (largest duplication) and safe shared utilities.

## Non-goals
- Any behavior changes to playback semantics.
- Major API redesign or new features.
- Reformatting the entire codebase.

## Constraints
- Work on branch: `refactor/dry-cleanup-1`
- Keep public REST and WS behavior stable.
- Keep generated OpenAPI / protobuf artifacts untouched unless absolutely necessary.

## Success criteria
- DRY reduction: consolidate duplicated streamer logic so that only one implementation contains the shared loop/broadcast scaffolding.
- All tests pass: `go test ./...`
- Lint passes (repo already has `.golangci.yml`): `golangci-lint run` if available; otherwise keep code lint-clean and let CI validate.
- `just` commands still work (don’t break dev workflow).

## Implementation tasks

### Task 1: Consolidate WebSocket streamer implementations
- [x] Introduce a parameterized/base streamer in `internal/ws/` (e.g., `base_streamer.go`) that contains the shared struct fields and `Run()` loop.
- [x] Replace `internal/ws/gex_streamer.go`, `classic_streamer.go`, `greek_streamer.go`, `greek_one_streamer.go`, and `streamer.go` duplication with thin config wrappers (or a single constructor) that supply:
  - group parsing function
  - package/category constants
  - encoder method (EncodeOrderflow / EncodeGex / EncodeGreek)
  - proto type URL
  - WS cache key prefix
- [x] Keep hub validators and group naming rules intact.

### Task 2: Consolidate ticker/category extraction helpers
- [x] Replace duplicated `extract*TickerAndCategory` functions with one generic helper accepting:
  - separator string
  - allowed categories set
- [x] Add unit tests for extraction parsing edge cases.

### Task 3: Small shared utilities (low risk)
- [x] Move identical timestamp extraction logic to a shared helper in `internal/data/` (or `internal/common/`) and reuse in both loaders.
- [x] Consolidate `getEnvOrDefault`/bool/duration parsing into a small `internal/envutil` package and update callers.

### Task 4: Verify behavior + tests
- [x] Run `go test ./...`
- [x] Run a quick sanity compile of `cmd/server`, `cmd/downloader`, `cmd/daemon`.
- [x] Ensure websocket server still starts (build only).

### Task 5: Commit + push
- [ ] Commit with message like `refactor(ws): consolidate streamers` and `refactor: shared env/timestamp helpers` (or a single commit).
- [ ] Push branch.

## Test plan
- `go test ./...`
- (Optional) `golangci-lint run` if installed

## Acceptance checklist
- [ ] WebSocket streamer duplication reduced substantially (no 5 nearly-identical streamer loops).
- [ ] Tests pass.
- [ ] Changes committed on `refactor/dry-cleanup-1`.
- [ ] Branch pushed to origin.
