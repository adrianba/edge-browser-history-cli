# AGENTS.md

## Implementation notes

- Runtime: Go module (`github.com/adrianba/edge-browser-history-cli`), built as a single self-contained native executable.
- Entry point is `main.go`, which delegates to `edge.Run`.
- Core logic lives in the `edge/` package:
  - argument parsing (`edge/cli.go`: `ParseArguments`, `CliArguments`, `HistoryRequest`),
  - command flow and JSON output (`edge/run.go`: `Run`),
  - profile discovery and history querying (`edge/history.go`: `ListProfiles`, `GetHistory`),
  - local-time range handling and Chromium timestamp conversion (`edge/timeconv.go`),
  - history DB snapshot copying (`edge/snapshot.go`),
  - user-facing error type (`edge/errors.go`: `HistoryError`),
  - version constant (`edge/version.go`).
- SQLite access uses the pure-Go driver `modernc.org/sqlite` (no CGO), so the project cross-compiles and ships without external dependencies.

## Safety and correctness

- Reads only from copied temp DB snapshots.
- Copies `History` plus optional `-journal`, `-wal`, `-shm` files.
- Resolves profile path under the configured Edge user-data directory and rejects traversal.
- Local date/time windows are interpreted in the machine local timezone with `time.Local` (DST-aware conversion to UTC before querying).

## CLI contract

- `--help` prints text help.
- `--profiles` outputs JSON list of profiles.
- `--history --profile <...> --date <yyyy-MM-dd>` outputs JSON entries.
- Optional `--start-time` / `--end-time` narrow results to local-time sub-range.
- All non-help output is JSON (including errors).

## Versioning and releases

- Version is stored in `edge/version.go` (the `Version` constant).
- CI workflow (`.github/workflows/ci.yml`) builds, vets, and tests on push/PR to `main`.
- Release workflow (`.github/workflows/release.yml`) bumps version, builds self-contained executables for win-x64 and win-arm64, and publishes a GitHub Release. No runtime needs to be installed on the target machine.

## Build and test

- Build: `go build ./...`
- Vet: `go vet ./...`
- Test: `go test ./...`

## Test strategy

- Tests are in `edge/edge_test.go` (package-internal tests).
- Includes argument parsing checks, time-range validation, and an integration-style history query test using a synthetic SQLite DB created with the same `modernc.org/sqlite` driver.
