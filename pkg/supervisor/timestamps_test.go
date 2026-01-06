package supervisor

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetFileMtime(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	mtime := GetFileMtime(testFile)
	if mtime.IsZero() {
		t.Error("Expected non-zero mtime")
	}

	noFile := GetFileMtime(filepath.Join(tmpDir, "noexist.txt"))
	if !noFile.IsZero() {
		t.Error("Expected zero mtime for non-existent file")
	}
}

func TestGetDirLatestMtime(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "file1.txt")
	os.WriteFile(file1, []byte("test1"), 0644)

	time.Sleep(10 * time.Millisecond)

	file2 := filepath.Join(tmpDir, "file2.txt")
	os.WriteFile(file2, []byte("test2"), 0644)

	latest := GetDirLatestMtime(tmpDir)
	if latest.IsZero() {
		t.Error("Expected non-zero latest mtime")
	}

	file2Mtime := GetFileMtime(file2)
	if !latest.Equal(file2Mtime) {
		t.Error("Latest mtime should match file2's mtime")
	}
}

func TestHasFileChanged(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	os.WriteFile(testFile, []byte("original"), 0644)
	originalMtime := GetFileMtime(testFile)

	if HasFileChanged(testFile, originalMtime) {
		t.Error("File should not have changed")
	}

	time.Sleep(10 * time.Millisecond)
	os.WriteFile(testFile, []byte("modified"), 0644)

	if !HasFileChanged(testFile, originalMtime) {
		t.Error("File should have changed")
	}
}

func TestWorkerRegistry(t *testing.T) {
	registry := NewWorkerRegistry()

	worker := &WorkerInstance{
		Name:   "test",
		Route:  "/test",
		PID:    12345,
		Port:   9001,
		Status: "healthy",
	}

	registry.Register(worker)

	retrieved, ok := registry.Get("test")
	if !ok {
		t.Error("Worker should exist")
	}
	if retrieved.Name != "test" {
		t.Error("Worker name mismatch")
	}

	workers := registry.List()
	if len(workers) != 1 {
		t.Errorf("Expected 1 worker, got %d", len(workers))
	}
	registry.Remove("test")
	_, ok = registry.Get("test")
	if ok {
		t.Error("Worker should not exist after removal")
	}
}

func TestCheckChanges(t *testing.T) {
	tmpDir := t.TempDir()

	binFile := filepath.Join(tmpDir, "worker")
	publicDir := filepath.Join(tmpDir, "public")
	privateDir := filepath.Join(tmpDir, "private")

	os.WriteFile(binFile, []byte("binary"), 0755)
	os.MkdirAll(publicDir, 0755)
	os.MkdirAll(privateDir, 0755)
	os.WriteFile(filepath.Join(publicDir, "style.css"), []byte("css"), 0644)

	registry := NewWorkerRegistry()

	worker := &WorkerInstance{
		Name:         "test",
		BinaryPath:   binFile,
		BinaryMtime:  GetFileMtime(binFile),
		PublicPath:   publicDir,
		PublicMtime:  GetDirLatestMtime(publicDir),
		PrivatePath:  privateDir,
		PrivateMtime: GetDirLatestMtime(privateDir),
	}

	registry.Register(worker)

	changed, changeType := registry.CheckChanges("test")
	if changed {
		t.Error("Should not have changes initially")
	}

	time.Sleep(10 * time.Millisecond)
	os.WriteFile(binFile, []byte("new binary"), 0755)

	changed, changeType = registry.CheckChanges("test")
	if !changed || changeType != "binary" {
		t.Errorf("Expected binary change, got: %v, %s", changed, changeType)
	}

	registry.UpdateMtimes("test")

	time.Sleep(10 * time.Millisecond)
	os.WriteFile(filepath.Join(publicDir, "app.js"), []byte("js"), 0644)

	changed, changeType = registry.CheckChanges("test")
	if !changed || changeType != "assets" {
		t.Errorf("Expected assets change, got: %v, %s", changed, changeType)
	}
}
