package edge

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ProfileInfo describes a single Edge browser profile.
type ProfileInfo struct {
	Name      string `json:"name"`
	Directory string `json:"directory"`
	IsDefault bool   `json:"isDefault"`
}

// HistoryEntry is a single browsing history visit.
type HistoryEntry struct {
	VisitTime  string `json:"visitTime"`
	URL        string `json:"url"`
	Title      string `json:"title"`
	VisitCount int    `json:"visitCount"`
	TypedCount int    `json:"typedCount"`
	Transition string `json:"transition"`
	URLID      int64  `json:"urlId"`
	VisitID    int64  `json:"visitId"`
}

const maxLocalStateSize int64 = 10 * 1024 * 1024 // 10 MB

var nonBrowsingProfiles = map[string]bool{
	"guest profile":  true,
	"system profile": true,
}

var transitionMap = map[int]string{
	0:  "link",
	1:  "typed",
	2:  "auto_bookmark",
	3:  "auto_subframe",
	4:  "manual_subframe",
	5:  "generated",
	6:  "auto_toplevel",
	7:  "form_submit",
	8:  "reload",
	9:  "keyword",
	10: "keyword_generated",
}

// ListProfiles enumerates the browsing-capable Edge profiles under the given
// user data directory.
func ListProfiles(userDataDir string) ([]ProfileInfo, error) {
	info, err := os.Stat(userDataDir)
	if err != nil || !info.IsDir() {
		return nil, newHistoryError("Edge User Data directory not found: " + userDataDir)
	}

	infoCache := loadInfoCache(filepath.Join(userDataDir, "Local State"))

	profiles := make([]ProfileInfo, 0)
	seenNames := make(map[string]int)

	directories := make([]string, 0, len(infoCache))
	for dir := range infoCache {
		directories = append(directories, dir)
	}
	sort.Strings(directories)

	for _, directory := range directories {
		if nonBrowsingProfiles[strings.ToLower(directory)] || !isSafeProfileDirectory(directory) {
			continue
		}

		historyPath := filepath.Join(userDataDir, directory, "History")
		if !fileExists(historyPath) {
			continue
		}

		friendlyName := infoCache[directory]
		if strings.TrimSpace(friendlyName) == "" {
			friendlyName = directory
		}
		seenNames[friendlyName]++
		profiles = append(profiles, ProfileInfo{
			Name:      friendlyName,
			Directory: directory,
			IsDefault: directory == "Default",
		})
	}

	if len(profiles) == 0 {
		entries, err := os.ReadDir(userDataDir)
		if err != nil {
			return nil, newHistoryError("Edge User Data directory not found: " + userDataDir)
		}

		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if e.IsDir() {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)

		for _, name := range names {
			if nonBrowsingProfiles[strings.ToLower(name)] || !isSafeProfileDirectory(name) {
				continue
			}
			if !fileExists(filepath.Join(userDataDir, name, "History")) {
				continue
			}
			profiles = append(profiles, ProfileInfo{
				Name:      name,
				Directory: name,
				IsDefault: name == "Default",
			})
		}

		return profiles, nil
	}

	for i := range profiles {
		if seenNames[profiles[i].Name] > 1 {
			profiles[i].Name = fmt.Sprintf("%s (%s)", profiles[i].Name, profiles[i].Directory)
		}
	}

	return profiles, nil
}

// GetHistory queries browsing history for the requested profile, date and
// optional local time range.
func GetHistory(ctx context.Context, userDataDir string, request *HistoryRequest) ([]HistoryEntry, error) {
	day, err := time.ParseInLocation(dateLayout, request.Date, time.Local)
	if err != nil {
		return nil, newHistoryError(fmt.Sprintf("Invalid date '%s'; expected yyyy-MM-dd.", request.Date))
	}

	profiles, err := ListProfiles(userDataDir)
	if err != nil {
		return nil, err
	}

	var profile *ProfileInfo
	for i := range profiles {
		if profiles[i].Name == request.Profile || profiles[i].Directory == request.Profile {
			profile = &profiles[i]
			break
		}
	}
	if profile == nil {
		return nil, newHistoryError(fmt.Sprintf("Unknown profile '%s'.", request.Profile))
	}

	startUTC, endUTC, err := timeRangeToUTC(day, request.StartTime, request.EndTime)
	if err != nil {
		return nil, err
	}
	startChrome := timeToChromeMicroseconds(startUTC)
	endChrome := timeToChromeMicroseconds(endUTC)

	baseDir, err := filepath.Abs(userDataDir)
	if err != nil {
		return nil, newHistoryError("Unable to resolve Edge User Data directory: " + err.Error())
	}
	historyPath, err := filepath.Abs(filepath.Join(baseDir, profile.Directory, "History"))
	if err != nil {
		return nil, newHistoryError("Unable to resolve history path: " + err.Error())
	}
	if !strings.HasPrefix(historyPath, baseDir) {
		return nil, newHistoryError("Resolved history path escapes the Edge User Data directory.")
	}

	snapshot, err := createHistorySnapshot(historyPath)
	if err != nil {
		return nil, err
	}
	defer snapshot.Close()

	return queryHistory(ctx, snapshot.HistoryPath, startChrome, endChrome)
}

func queryHistory(ctx context.Context, dbPath string, startChrome, endChrome int64) ([]HistoryEntry, error) {
	dsn := fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(5000)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, newHistoryError("Unable to open Edge history database: " + err.Error())
	}
	defer db.Close()

	const query = `
		SELECT v.id, v.visit_time, v.transition,
		       u.id, u.url, u.title, u.visit_count, u.typed_count
		FROM visits v
		JOIN urls u ON v.url = u.id
		WHERE v.visit_time >= ? AND v.visit_time < ?
		ORDER BY v.visit_time ASC`

	rows, err := db.QueryContext(ctx, query, startChrome, endChrome)
	if err != nil {
		return nil, newHistoryError("Unable to query Edge history database: " + err.Error())
	}
	defer rows.Close()

	entries := make([]HistoryEntry, 0)
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		var (
			visitID    int64
			visitTime  int64
			transition sql.NullInt64
			urlID      int64
			url        sql.NullString
			title      sql.NullString
			visitCount sql.NullInt64
			typedCount sql.NullInt64
		)

		if err := rows.Scan(&visitID, &visitTime, &transition, &urlID, &url, &title, &visitCount, &typedCount); err != nil {
			return nil, newHistoryError("Unable to read Edge history database: " + err.Error())
		}

		localTime, ok := chromeMicrosecondsToLocal(visitTime)
		if !ok {
			continue // skip entries with corrupt timestamps
		}

		entries = append(entries, HistoryEntry{
			VisitTime:  localTime.Format(visitTimeLayout),
			URL:        url.String,
			Title:      title.String,
			VisitCount: int(visitCount.Int64),
			TypedCount: int(typedCount.Int64),
			Transition: decodeTransition(int(transition.Int64)),
			URLID:      urlID,
			VisitID:    visitID,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, newHistoryError("Unable to read Edge history database: " + err.Error())
	}

	return entries, nil
}

func loadInfoCache(localStatePath string) map[string]string {
	result := make(map[string]string)

	info, err := os.Stat(localStatePath)
	if err != nil || info.Size() > maxLocalStateSize {
		return result
	}

	data, err := os.ReadFile(localStatePath)
	if err != nil {
		return result
	}

	var parsed struct {
		Profile struct {
			InfoCache map[string]struct {
				Name string `json:"name"`
			} `json:"info_cache"`
		} `json:"profile"`
	}

	if err := json.Unmarshal(data, &parsed); err != nil {
		return result
	}

	for dir, value := range parsed.Profile.InfoCache {
		result[dir] = value.Name
	}

	return result
}

func isSafeProfileDirectory(directory string) bool {
	if strings.TrimSpace(directory) == "" || directory == "." || directory == ".." {
		return false
	}
	if strings.ContainsAny(directory, "/\\") {
		return false
	}
	return !filepath.IsAbs(directory)
}

func decodeTransition(transition int) string {
	core := transition & 0xFF
	if decoded, ok := transitionMap[core]; ok {
		return decoded
	}
	return "unknown"
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
