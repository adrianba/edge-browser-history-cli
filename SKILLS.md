# SKILLS.md

## Analyze Edge browsing history with the CLI

### 1) Discover profiles

```bash
dotnet run --project /home/runner/work/edge-browser-history-cli/edge-browser-history-cli/EdgeBrowserHistoryCli -- --profiles
```

Use a returned `name` (or `directory`) as the `--profile` value.

### 2) Query a full day

```bash
dotnet run --project /home/runner/work/edge-browser-history-cli/edge-browser-history-cli/EdgeBrowserHistoryCli -- --history --profile "Personal" --date 2026-06-20
```

### 3) Query a focused local time range

```bash
dotnet run --project /home/runner/work/edge-browser-history-cli/edge-browser-history-cli/EdgeBrowserHistoryCli -- --history --profile "Personal" --date 2026-06-20 --start-time 09:00 --end-time 11:00
```

### 4) Use alternate Edge data path

```bash
dotnet run --project /home/runner/work/edge-browser-history-cli/edge-browser-history-cli/EdgeBrowserHistoryCli -- --profiles --user-data-dir "C:\\Users\\you\\AppData\\Local\\Microsoft\\Edge\\User Data"
```

All command output (except `--help`) is JSON, so it can be piped to tools like `jq`.
