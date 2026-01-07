package supervisor

import (
	"os"
	"time"
)

// GetFileMtime returns the modification time of a file.
// Returns zero time if file doesn't exist or on error.
func GetFileMtime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// HasFileChanged checks if a file's mtime is newer than the recorded time.
func HasFileChanged(path string, recordedTime time.Time) bool {
	currentMtime := GetFileMtime(path)
	if currentMtime.IsZero() {
		return false // File doesn't exist or error
	}
	return currentMtime.After(recordedTime)
}
