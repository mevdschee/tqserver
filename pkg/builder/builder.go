package builder

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type BuildResult struct {
	WorkerName string
	Success    bool
	Error      error
	OutputPath string
}

type Builder struct {
	workersDir string
	serverDir  string
}

func NewBuilder(workersDir, serverDir string) *Builder {
	return &Builder{
		workersDir: workersDir,
		serverDir:  serverDir,
	}
}

func (b *Builder) BuildWorker(workerName string) (*BuildResult, error) {
	result := &BuildResult{
		WorkerName: workerName,
	}

	workerDir := filepath.Join(b.workersDir, workerName)
	srcDir := filepath.Join(workerDir, "src")
	binDir := filepath.Join(workerDir, "bin")
	outputFile := filepath.Join(binDir, workerName)

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		result.Error = fmt.Errorf("source directory not found: %s", srcDir)
		return result, result.Error
	}

	if err := os.MkdirAll(binDir, 0755); err != nil {
		result.Error = fmt.Errorf("failed to create bin directory: %w", err)
		return result, result.Error
	}

	log.Printf("Building worker: %s", workerName)

	cmd := exec.Command("go", "build", "-o", outputFile, srcDir)
	cmd.Dir = workerDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Error = fmt.Errorf("build failed: %w\n%s", err, string(output))
		log.Printf("Build error for %s: %v", workerName, result.Error)
		return result, result.Error
	}

	result.Success = true
	result.OutputPath = outputFile
	log.Printf("Successfully built worker: %s -> %s", workerName, outputFile)

	return result, nil
}

func (b *Builder) BuildServer() error {
	srcDir := filepath.Join(b.serverDir, "src")
	binDir := filepath.Join(b.serverDir, "bin")
	outputFile := filepath.Join(binDir, "tqserver")

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("server source directory not found: %s", srcDir)
	}

	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	log.Println("Building server...")

	mainFiles, err := filepath.Glob(filepath.Join(srcDir, "*.go"))
	if err != nil {
		return fmt.Errorf("failed to find main files: %w", err)
	}

	if len(mainFiles) == 0 {
		return fmt.Errorf("no Go files found in %s", srcDir)
	}

	args := []string{"build", "-o", outputFile}
	args = append(args, mainFiles...)

	cmd := exec.Command("go", args...)
	cmd.Dir = b.serverDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("server build failed: %w\n%s", err, string(output))
	}

	log.Printf("Successfully built server: %s", outputFile)
	return nil
}

func (b *Builder) ListWorkers() ([]string, error) {
	entries, err := os.ReadDir(b.workersDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read workers directory: %w", err)
	}

	var workers []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		srcDir := filepath.Join(b.workersDir, entry.Name(), "src")
		mainFile := filepath.Join(srcDir, "main.go")

		if _, err := os.Stat(mainFile); err == nil {
			workers = append(workers, entry.Name())
		}
	}

	return workers, nil
}

func (b *Builder) BuildAll() error {
	workers, err := b.ListWorkers()
	if err != nil {
		return err
	}

	log.Printf("Building %d workers...", len(workers))

	var errors []string
	for _, worker := range workers {
		if _, err := b.BuildWorker(worker); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", worker, err))
		}
	}

	if err := b.BuildServer(); err != nil {
		errors = append(errors, fmt.Sprintf("server: %v", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("build errors:\n%s", strings.Join(errors, "\n"))
	}

	log.Println("All builds completed successfully")
	return nil
}
