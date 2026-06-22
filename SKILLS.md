---
name: Edge Browser History CLI
id: edge-browser-history-cli
description: |
  Query Microsoft Edge browsing history from the command line.
  Supports profile discovery, date-based history queries, and
  local time range filtering. All output is JSON.
version: 0.1.0
---

# Edge Browser History CLI

Analyze Edge browsing history with the CLI.

## 1) Discover profiles

```bash
dotnet run --project EdgeBrowserHistoryCli -- --profiles
```

Use a returned `name` (or `directory`) as the `--profile` value.

## 2) Query a full day

```bash
dotnet run --project EdgeBrowserHistoryCli -- --history --profile "Personal" --date 2026-06-20
```

## 3) Query a focused local time range

```bash
dotnet run --project EdgeBrowserHistoryCli -- --history --profile "Personal" --date 2026-06-20 --start-time 09:00 --end-time 11:00
```

## 4) Use alternate Edge data path

```bash
dotnet run --project EdgeBrowserHistoryCli -- --profiles --user-data-dir "C:\\Users\\you\\AppData\\Local\\Microsoft\\Edge\\User Data"
```

All command output (except `--help`) is JSON, so it can be piped to tools like `jq`.
