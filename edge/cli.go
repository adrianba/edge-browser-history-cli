package edge

import "strings"

// HistoryRequest describes a request for browsing history for a single profile
// and local date, optionally narrowed to a local time sub-range.
type HistoryRequest struct {
	Profile   string
	Date      string
	StartTime string
	EndTime   string
}

// CliArguments is the parsed representation of the command-line arguments.
type CliArguments struct {
	ListProfiles   bool
	HistoryRequest *HistoryRequest
	UserDataDir    string
}

// ParseArguments parses the raw command-line arguments (excluding the program
// name) into a CliArguments value.
func ParseArguments(args []string) (*CliArguments, error) {
	values := make(map[string]string)
	flags := make(map[string]bool)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			return nil, newHistoryError("Unexpected argument: " + arg)
		}

		lower := strings.ToLower(arg)
		if lower == "--profiles" || lower == "--history" {
			flags[lower] = true
			continue
		}

		if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
			return nil, newHistoryError("Missing value for " + arg + ".")
		}

		i++
		values[lower] = args[i]
	}

	hasProfiles := flags["--profiles"]
	hasHistory := flags["--history"]

	// Infer --history mode when --profile or --date are provided without an
	// explicit mode flag.
	if !hasProfiles && !hasHistory {
		if values["--profile"] != "" || values["--date"] != "" {
			hasHistory = true
		}
	}

	if hasProfiles == hasHistory {
		return nil, newHistoryError("Specify exactly one of --profiles or --history.")
	}

	userDataDir := values["--user-data-dir"]

	if hasProfiles {
		return &CliArguments{ListProfiles: true, UserDataDir: userDataDir}, nil
	}

	profile := values["--profile"]

	date, ok := values["--date"]
	if !ok || strings.TrimSpace(date) == "" {
		return nil, newHistoryError("--history requires --date in yyyy-MM-dd format.")
	}

	return &CliArguments{
		ListProfiles: false,
		HistoryRequest: &HistoryRequest{
			Profile:   profile,
			Date:      date,
			StartTime: values["--start-time"],
			EndTime:   values["--end-time"],
		},
		UserDataDir: userDataDir,
	}, nil
}
