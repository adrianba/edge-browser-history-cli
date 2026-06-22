# AGENTS.md

## Implementation notes

- Runtime: .NET 8 console app in `/home/runner/work/edge-browser-history-cli/edge-browser-history-cli/EdgeBrowserHistoryCli`.
- Primary flow lives in `Program.cs`:
  - argument parsing (`CliArguments`),
  - profile discovery (`EdgeHistoryService.ListProfiles`),
  - history querying (`EdgeHistoryService.GetHistoryAsync`),
  - local-time range handling (`LocalTimeRange`),
  - Chromium timestamp conversion (`DateTimeConverters`),
  - history DB snapshot copying (`HistorySnapshot`).

## Safety and correctness

- Reads only from copied temp DB snapshots.
- Copies `History` plus optional `-journal`, `-wal`, `-shm` files.
- Resolves profile path under the configured Edge user-data directory and rejects traversal.
- Local date/time windows are interpreted in machine local timezone with `TimeZoneInfo.Local` (DST-aware conversion to UTC before querying).

## CLI contract

- `--help` prints text help.
- `--profiles` outputs JSON list of profiles.
- `--history --profile <...> --date <yyyy-MM-dd>` outputs JSON entries.
- Optional `--start-time` / `--end-time` narrow results to local-time sub-range.
- All non-help output is JSON (including errors).

## Test strategy

- Tests are in `/home/runner/work/edge-browser-history-cli/edge-browser-history-cli/EdgeBrowserHistoryCli.Tests`.
- Includes argument parsing checks, time-range validation, and integration-style history query test using a synthetic SQLite DB.
