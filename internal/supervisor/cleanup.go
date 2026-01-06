package supervisor

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

// cleanupOldBinaries removes old worker binaries from the temp directory
func (s *Supervisor) cleanupOldBinaries() {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.performCleanup()
		}
	}
}

// performCleanup removes binaries older than 24 hours
func (s *Supervisor) performCleanup() {
	tempDir := s.config.Workers.TempDir
	cutoff := time.Now().Add(-24 * time.Hour)

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		log.Printf("Failed to read temp directory for cleanup: %v", err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(tempDir, entry.Name())
			if err := os.Remove(path); err != nil {
				log.Printf("Failed to remove old binary %s: %v", path, err)
			} else {
				log.Printf("Cleaned up old binary: %s", path)
			}
		}
	}
}
