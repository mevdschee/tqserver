# Binary Cleanup Strategy

To keep the deployment clean and prevent disk usage build-up, TQServer implements a strict binary cleanup strategy for compiled workers (Go).

## Single Active Binary

Unlike some systems that generate unique binaries for every build (e.g., `worker-v1`, `worker-v2`), TQServer uses a **fixed binary naming convention**:

-   **Go**: The binary is always named exactly after the worker (e.g., `api`, `inventory`, `mailer`).
-   **Location**: Always stored in `workers/{name}/bin/`.

## Cleanup Process

The build process in `Supervisor.buildWorker` handles cleanup automatically:

1.  **Fixed Path**: The target output path is always `workers/{name}/bin/{name}`.
2.  **Overwrite**: The `go build` command overwrites the existing binary at that path.
3.  **No Stale Binaries**: Since the filename is constant, there are no old versions (e.g., `worker-12345`) left accumulating in the `bin` directory.

## Implementation Details

```go
// From Supervisor.buildWorker
binaryName := worker.Name
binaryPath = filepath.Join(workerBinDir, binaryName)

// ...

// Build directly to the fixed path, overwriting any previous version
cmd = exec.Command("go", "build", "-o", binaryPath)
```

This strategy ensures that the `bin` directory contains **only the currently active binary** for the worker, significantly reducing disk clutter during long-running development sessions or in production environments with frequent deployments.
