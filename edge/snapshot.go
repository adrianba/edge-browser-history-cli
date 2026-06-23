package edge

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// historySnapshot is a temporary copy of an Edge History database (plus its
// sidecar files) used so that the live database is never read directly.
type historySnapshot struct {
	tempDir     string
	HistoryPath string
}

// createHistorySnapshot copies the source History database and any of its
// sidecar files into a fresh temp directory and returns a snapshot describing
// the copy. Callers must invoke Close to remove the temp directory.
func createHistorySnapshot(sourceHistoryPath string) (*historySnapshot, error) {
	tempDir, err := os.MkdirTemp("", "edge_history_cli_")
	if err != nil {
		return nil, newHistoryError("Unable to create temp directory: " + err.Error())
	}

	copiedHistoryPath := filepath.Join(tempDir, "History")

	if err := copyFile(sourceHistoryPath, copiedHistoryPath); err != nil {
		os.RemoveAll(tempDir)
		return nil, newHistoryError("Unable to copy Edge history database: " + err.Error())
	}

	for _, suffix := range []string{"-journal", "-wal", "-shm"} {
		sourceSidecar := sourceHistoryPath + suffix
		if _, statErr := os.Stat(sourceSidecar); statErr == nil {
			if err := copyFile(sourceSidecar, copiedHistoryPath+suffix); err != nil {
				os.RemoveAll(tempDir)
				return nil, newHistoryError("Unable to copy Edge history database: " + err.Error())
			}
		}
	}

	return &historySnapshot{tempDir: tempDir, HistoryPath: copiedHistoryPath}, nil
}

// Close removes the temporary directory backing the snapshot.
func (s *historySnapshot) Close() {
	if err := os.RemoveAll(s.tempDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to clean up temp directory '%s': %v\n", s.tempDir, err)
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}

	return out.Close()
}
