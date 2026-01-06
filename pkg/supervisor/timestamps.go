package supervisor

import (
	"os"
	"path/filepath"
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

// GetDirLatestMtime returns the latest modification time of any file in a directory (recursive).
// Returns zero time if directory is empty or doesn't exist.
func GetDirLatestMtime(dirPath string) time.Time {
	var latest time.Time

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}
		if info.IsDir() {
			return nil // Skip directories themselves
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		return nil
	})

	if err != nil {
		return time.Time{}
	}

	return latest
}

// HasFileChanged checks if a file's mtime is newer than the recorded time.
func HasFileChanged(path string, recordedTime time.Time) bool {
	currentMtime := GetFileMtime(path)
	if currentMtime.IsZero() {
		return false // File doesn't exist or error
	}
	return currentMtime.After(recordedTime)
}

// HasDirChanged checks if any file in a directory has changed since the recorded time.
func HasDirChanged(dirPath string, recordedTime time.Time) bool {
	latestMtime := GetDirLatestMtime(dirPath)
	if latestMtime.IsZero() {
		return false // Directory empty or doesn't exist
	}
	return latestMtime.After(recordedTime)
}
