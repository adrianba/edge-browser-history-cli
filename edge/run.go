package edge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
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

	// If no profile was specified, use the default profile.
	if strings.TrimSpace(parsed.HistoryRequest.Profile) == "" {
		profiles, err := ListProfiles(userDataDir)
		if err != nil {
			return writeError(stdout, err)
		}
		defaultProfile := ""
		for _, p := range profiles {
			if p.IsDefault {
				defaultProfile = p.Name
				break
			}
		}
		if defaultProfile == "" && len(profiles) > 0 {
			defaultProfile = profiles[0].Name
		}
		if defaultProfile == "" {
			return writeError(stdout, newHistoryError("No profiles found."))
		}
		parsed.HistoryRequest.Profile = defaultProfile
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
		"  edge-browser-history-cli --history [--profile <name-or-directory>] --date <yyyy-MM-dd> [--start-time <HH:mm[:ss]>] [--end-time <HH:mm[:ss]>] [--user-data-dir <path>]",
		"",
		"Options:",
		"  --help                 Show this help text.",
		"  --profiles             List available Edge browser profiles.",
		"  --history              Return browsing history for a given profile and date.",
		"  --profile              Profile name or directory id (e.g. Default, Profile 1). Defaults to the default profile.",
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
	data = escapeNonASCII(data)
	fmt.Fprintln(w, string(data))
}

// escapeNonASCII replaces all non-ASCII bytes in JSON output with \uXXXX escape
// sequences. This ensures the JSON is pure ASCII, which prevents PowerShell from
// misinterpreting the output when [Console]::OutputEncoding is set to a non-UTF-8
// code page (the default on many Windows systems).
func escapeNonASCII(data []byte) []byte {
	// Fast path: if all bytes are ASCII, return as-is.
	allASCII := true
	for _, b := range data {
		if b >= 0x80 {
			allASCII = false
			break
		}
	}
	if allASCII {
		return data
	}

	var buf bytes.Buffer
	buf.Grow(len(data))
	for i := 0; i < len(data); {
		b := data[i]
		if b < 0x80 {
			buf.WriteByte(b)
			i++
		} else {
			r, size := utf8.DecodeRune(data[i:])
			if r == utf8.RuneError && size == 1 {
				// Invalid UTF-8 byte; emit replacement character escape.
				buf.WriteString(`\ufffd`)
				i++
			} else if r <= 0xFFFF {
				fmt.Fprintf(&buf, `\u%04x`, r)
				i += size
			} else {
				// Encode as a UTF-16 surrogate pair for characters above BMP.
				r -= 0x10000
				hi := 0xD800 + ((r >> 10) & 0x3FF)
				lo := 0xDC00 + (r & 0x3FF)
				fmt.Fprintf(&buf, `\u%04x\u%04x`, hi, lo)
				i += size
			}
		}
	}
	return buf.Bytes()
}

func writeError(w io.Writer, err error) int {
	writeJSON(w, map[string]string{"error": err.Error()})
	return 1
}
