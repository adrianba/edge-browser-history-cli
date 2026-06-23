package edge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Run executes the CLI flow and returns the process exit code. All non-help
// output (including errors) is written as JSON to stdout.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || containsHelp(args) {
		printHelp(stdout)
		return 0
	}

	parsed, err := ParseArguments(args)
	if err != nil {
		return writeError(stdout, err)
	}

	userDataDir, err := resolveUserDataDir(parsed.UserDataDir)
	if err != nil {
		return writeError(stdout, err)
	}

	if parsed.ListProfiles {
		profiles, err := ListProfiles(userDataDir)
		if err != nil {
			return writeError(stdout, err)
		}
		writeJSON(stdout, map[string]any{"profiles": profiles})
		return 0
	}

	if parsed.HistoryRequest == nil {
		return writeError(stdout, newHistoryError("No command specified. Use --profiles or --history."))
	}

	entries, err := GetHistory(ctx, userDataDir, parsed.HistoryRequest)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return writeError(stdout, newHistoryError("Operation timed out."))
		}
		return writeError(stdout, err)
	}

	writeJSON(stdout, historyOutput{
		Profile:   parsed.HistoryRequest.Profile,
		Date:      parsed.HistoryRequest.Date,
		StartTime: parsed.HistoryRequest.StartTime,
		EndTime:   parsed.HistoryRequest.EndTime,
		Entries:   entries,
	})
	return 0
}

// historyOutput is the JSON shape returned for a --history command. startTime
// and endTime are omitted when empty.
type historyOutput struct {
	Profile   string         `json:"profile"`
	Date      string         `json:"date"`
	StartTime string         `json:"startTime,omitempty"`
	EndTime   string         `json:"endTime,omitempty"`
	Entries   []HistoryEntry `json:"entries"`
}

func containsHelp(args []string) bool {
	for _, a := range args {
		if strings.EqualFold(a, "--help") {
			return true
		}
	}
	return false
}

func resolveUserDataDir(override string) (string, error) {
	var resolved string

	switch {
	case strings.TrimSpace(override) != "":
		resolved = override
	case strings.TrimSpace(os.Getenv("EDGE_USER_DATA_DIR")) != "":
		resolved = os.Getenv("EDGE_USER_DATA_DIR")
	default:
		localAppData := os.Getenv("LOCALAPPDATA")
		if strings.TrimSpace(localAppData) == "" {
			return "", newHistoryError("LOCALAPPDATA is not set; cannot locate Edge data. Set EDGE_USER_DATA_DIR to override.")
		}
		resolved = filepath.Join(localAppData, "Microsoft", "Edge", "User Data")
	}

	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", newHistoryError("Edge User Data directory not found: " + resolved)
	}

	return resolved, nil
}

func printHelp(w io.Writer) {
	lines := []string{
		fmt.Sprintf("Edge Browser History CLI v%s", Version),
		"",
		"Usage:",
		"  edge-browser-history-cli --help",
		"  edge-browser-history-cli --profiles [--user-data-dir <path>]",
		"  edge-browser-history-cli --history --profile <name-or-directory> --date <yyyy-MM-dd> [--start-time <HH:mm[:ss]>] [--end-time <HH:mm[:ss]>] [--user-data-dir <path>]",
		"",
		"Options:",
		"  --help                 Show this help text.",
		"  --profiles             List available Edge browser profiles.",
		"  --history              Return browsing history for a given profile and date.",
		"  --profile              Profile name or directory id (e.g. Default, Profile 1).",
		"  --date                 Local date in yyyy-MM-dd format.",
		"  --start-time           Optional local start time in HH:mm or HH:mm:ss.",
		"  --end-time             Optional local end time in HH:mm or HH:mm:ss (exclusive). Must be after start-time.",
		"  --user-data-dir        Optional override for Edge User Data directory.",
	}
	fmt.Fprintln(w, strings.Join(lines, "\n"))
}

// orderedHistoryOutput build helpers removed in favor of the historyOutput struct.

func writeJSON(w io.Writer, payload any) {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error serializing output:", err)
		return
	}
	fmt.Fprintln(w, string(data))
}

func writeError(w io.Writer, err error) int {
	writeJSON(w, map[string]string{"error": err.Error()})
	return 1
}
