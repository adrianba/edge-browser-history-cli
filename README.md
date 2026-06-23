# edge-browser-history-cli

`edge-browser-history-cli` is a Go command-line tool for reading Microsoft Edge browsing history.

It follows the same approach as `adrianba/edge-browser-mcp`: copy the live `History` SQLite database (and sidecar files) to a temp location, then query the copy.

It is distributed as a single self-contained native executable with no runtime dependencies, giving fast startup and easy deployment.

## Features

- `--help`: shows command help.
- `--profiles`: lists Edge browser profiles.
- `--history`: returns history entries for a profile and local date (`yyyy-MM-dd`).
- Optional local time range filtering with `--start-time` and `--end-time`.
- Local date/time handling uses the machine timezone (DST-aware).
- All output except `--help` is JSON.

## Build

```bash
go build ./...
go test ./...
```

Build a standalone executable:

```bash
go build -o edge-browser-history-cli .
```

## Usage

```bash
# help (text)
go run . --help

# list profiles (JSON)
go run . --profiles

# history for a day (JSON)
go run . --history --profile "Default" --date 2026-06-20

# history in a local time range (JSON)
go run . --history --profile "Default" --date 2026-06-20 --start-time 09:00 --end-time 10:30
```

## CI/CD

- **CI**: On push to `main` and pull requests, the project is automatically built, vetted, and tested via GitHub Actions (`.github/workflows/ci.yml`).
- **Release**: A manual workflow (`.github/workflows/release.yml`) bumps the version, builds standalone executables for `win-x64` and `win-arm64`, and creates a GitHub Release with the artifacts.

## Prerequisites

Release artifacts are statically linked, self-contained native executables (`.exe` on Windows) and have no runtime dependencies.

## Versioning

The application version is stored in `edge/version.go` (the `Version` constant). It is updated automatically by the release workflow.

## Edge data location

By default, the CLI reads:

- `%LOCALAPPDATA%\\Microsoft\\Edge\\User Data`

You can override with:

- `EDGE_USER_DATA_DIR` environment variable, or
- `--user-data-dir <path>`.

## JSON output examples

`--profiles`

```json
{
  "profiles": [
    {
      "name": "Personal",
      "directory": "Default",
      "isDefault": true
    }
  ]
}
```

`--history`

```json
{
  "profile": "Personal",
  "date": "2026-06-20",
  "startTime": "09:00",
  "endTime": "10:30",
  "entries": [
    {
      "visitTime": "2026-06-20T09:03:12.1234567-07:00",
      "url": "https://example.com",
      "title": "Example",
      "visitCount": 1,
      "typedCount": 0,
      "transition": "link",
      "urlId": 123,
      "visitId": 456
    }
  ]
}
```
