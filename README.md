# edge-browser-history-cli

`edge-browser-history-cli` is a .NET command-line tool (modern C#) for reading Microsoft Edge browsing history.

It follows the same approach as `adrianba/edge-browser-mcp`: copy the live `History` SQLite database (and sidecar files) to a temp location, then query the copy.

## Features

- `--help`: shows command help.
- `--profiles`: lists Edge browser profiles.
- `--history`: returns history entries for a profile and local date (`yyyy-MM-dd`).
- Optional local time range filtering with `--start-time` and `--end-time`.
- Local date/time handling uses the machine timezone (DST-aware).
- All output except `--help` is JSON.

## Build

```bash
dotnet build /home/runner/work/edge-browser-history-cli/edge-browser-history-cli/edge-browser-history-cli.slnx
```

## Usage

```bash
# help (text)
dotnet run --project /home/runner/work/edge-browser-history-cli/edge-browser-history-cli/EdgeBrowserHistoryCli -- --help

# list profiles (JSON)
dotnet run --project /home/runner/work/edge-browser-history-cli/edge-browser-history-cli/EdgeBrowserHistoryCli -- --profiles

# history for a day (JSON)
dotnet run --project /home/runner/work/edge-browser-history-cli/edge-browser-history-cli/EdgeBrowserHistoryCli -- --history --profile "Default" --date 2026-06-20

# history in a local time range (JSON)
dotnet run --project /home/runner/work/edge-browser-history-cli/edge-browser-history-cli/EdgeBrowserHistoryCli -- --history --profile "Default" --date 2026-06-20 --start-time 09:00 --end-time 10:30
```

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
