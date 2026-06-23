package edge

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestParseProfilesCommand_Works(t *testing.T) {
	parsed, err := ParseArguments([]string{"--profiles", "--user-data-dir", "/tmp/edge"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !parsed.ListProfiles {
		t.Errorf("expected ListProfiles to be true")
	}
	if parsed.HistoryRequest != nil {
		t.Errorf("expected HistoryRequest to be nil")
	}
	if parsed.UserDataDir != "/tmp/edge" {
		t.Errorf("expected UserDataDir to be /tmp/edge, got %q", parsed.UserDataDir)
	}
}

func TestParseImplicitHistory_InfersHistoryMode(t *testing.T) {
	parsed, err := ParseArguments([]string{"--profile", "Profile 1", "--date", "2026-06-22"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.ListProfiles {
		t.Errorf("expected ListProfiles to be false")
	}
	if parsed.HistoryRequest == nil {
		t.Fatalf("expected HistoryRequest to be non-nil")
	}
	if parsed.HistoryRequest.Profile != "Profile 1" {
		t.Errorf("expected profile 'Profile 1', got %q", parsed.HistoryRequest.Profile)
	}
	if parsed.HistoryRequest.Date != "2026-06-22" {
		t.Errorf("expected date '2026-06-22', got %q", parsed.HistoryRequest.Date)
	}
}

func TestParseHistoryWithoutProfile_Throws(t *testing.T) {
	_, err := ParseArguments([]string{"--history", "--date", "2026-01-01"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "--profile") {
		t.Errorf("expected error to mention --profile, got %q", err.Error())
	}
}

func TestLocalTimeRange_InvalidRange_Throws(t *testing.T) {
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	_, _, err := timeRangeToUTC(day, "10:00", "09:59")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Invalid time range") {
		t.Errorf("expected error to mention Invalid time range, got %q", err.Error())
	}
}

func TestHistoryQuery_FiltersByProfileDateAndTimeRange(t *testing.T) {
	root := t.TempDir()

	defaultDir := filepath.Join(root, "Default")
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		t.Fatal(err)
	}

	localState := `{
  "profile": {
    "info_cache": {
      "Default": { "name": "Personal" }
    }
  }
}`
	if err := os.WriteFile(filepath.Join(root, "Local State"), []byte(localState), 0o644); err != nil {
		t.Fatal(err)
	}

	historyPath := filepath.Join(defaultDir, "History")
	createHistoryDB(t, historyPath, time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local))

	profiles, err := ListProfiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}
	if profiles[0].Name != "Personal" {
		t.Errorf("expected profile name Personal, got %q", profiles[0].Name)
	}

	entries, err := GetHistory(context.Background(), root, &HistoryRequest{
		Profile:   "Personal",
		Date:      "2026-01-01",
		StartTime: "09:00",
		EndTime:   "10:00",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].URL != "https://example.com/one" {
		t.Errorf("expected url https://example.com/one, got %q", entries[0].URL)
	}
}

func createHistoryDB(t *testing.T, path string, day time.Time) {
	t.Helper()

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	schema := `
		CREATE TABLE urls (
			id INTEGER PRIMARY KEY,
			url LONGVARCHAR,
			title LONGVARCHAR,
			visit_count INTEGER,
			typed_count INTEGER
		);
		CREATE TABLE visits (
			id INTEGER PRIMARY KEY,
			url INTEGER,
			visit_time INTEGER,
			transition INTEGER
		);`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}

	insertVisit(t, db, 1, "https://example.com/one", "One", localToChrome(day, 9, 30, 0), 1)
	insertVisit(t, db, 2, "https://example.com/two", "Two", localToChrome(day, 11, 0, 0), 8)
}

func insertVisit(t *testing.T, db *sql.DB, id int, url, title string, visitTime int64, transition int) {
	t.Helper()
	if _, err := db.Exec(
		"INSERT INTO urls(id, url, title, visit_count, typed_count) VALUES(?, ?, ?, 1, 0)",
		id, url, title); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		"INSERT INTO visits(id, url, visit_time, transition) VALUES(?, ?, ?, ?)",
		id, id, visitTime, transition); err != nil {
		t.Fatal(err)
	}
}

func localToChrome(day time.Time, hour, minute, second int) int64 {
	local := time.Date(day.Year(), day.Month(), day.Day(), hour, minute, second, 0, time.Local)
	return timeToChromeMicroseconds(local)
}
