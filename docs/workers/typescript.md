# TypeScript Workers Guide (Bun)

This guide explains how to create and integrate TypeScript-based workers with TQServer using the Bun runtime.

## Table of Contents

- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Getting Started](#getting-started)
- [Project Structure](#project-structure)
- [Creating a TypeScript Worker](#creating-a-typescript-worker)
- [Configuration](#configuration)
- [Auto-Scaling](#auto-scaling)
- [Development Workflow](#development-workflow)
- [Best Practices](#best-practices)

## Introduction

TQServer supports high-performance TypeScript workers using the [Bun](https://bun.sh) runtime. Bun provides a fast JavaScript executable, a package manager, and a test runner, all in one.

Features:
- **Fast Startup**: Bun starts significantly faster than Node.js.
- **Built-in TypeScript**: No compilation step required; Bun runs `.ts` files directly.
- **Hot Reload**: TQServer automatically detects file changes and reloads your worker instantly.
- **Auto-Scaling**: Configure workers to scale up based on load and scale down when idle.

## Prerequisites

- **Bun**: Install Bun (v1.0+)
  ```bash
  curl -fsSL https://bun.sh/install | bash
  ```
- **TQServer**: Latest version

## Getting Started

### Quick Start

1. **Navigate to workers directory:**
   ```bash
   cd workers
   ```

2. **Create a new worker directory:**
   ```bash
   mkdir my-worker
   cd my-worker
   ```

3. **Initialize project:**
   ```bash
   bun init -y
   bun add express
   bun add -d @types/express
   ```

4. **Create `config/worker.yaml`:**
   ```bash
   mkdir config
   nano config/worker.yaml
   ```
   Add configuration:
   ```yaml
   path: "/my-worker"
   type: "bun"
   enabled: true
   ```

5. **Start TQServer:**
   TQServer will automatically detect the new worker, install dependencies, and start it.

## Project Structure

A typical Bun worker structure:

```
workers/
  your-worker/
    ├── config/
    │   └── worker.yaml        # Worker configuration
    ├── node_modules/          # Dependencies (managed by Bun)
    ├── index.ts               # Main entry point
    ├── package.json           # Project metadata and dependencies
    ├── bun.lockb              # Lockfile
    └── tsconfig.json          # TypeScript config (optional)
```

## Creating a TypeScript Worker

Your `index.ts` should start an HTTP server (typically using Express) listening on the port provided by the `PORT` or `WORKER_PORT` environment variable.

### Example `index.ts`

```typescript
import express from 'express';

const app = express();
const port = process.env.PORT || 3000;
const workerName = process.env.WORKER_NAME || 'unknown';

app.use(express.json());

// Health check (Required)
app.get('/health', (req, res) => {
    res.json({ status: 'ok' });
});

// Main Route
app.get('/', (req, res) => {
    res.json({ 
        message: 'Hello from Bun!',
        worker: workerName 
    });
});

app.listen(port, () => {
    console.log(`Worker listening on port ${port}`);
});
```

## Configuration

### Worker Configuration (`config/worker.yaml`)

```yaml
# Path prefix
path: "/api"

# Worker type
type: "bun"

# Enabled status
enabled: true

# Bun specifc configuration
bun:
  entrypoint: "src/server.ts"  # Default is "index.ts"
  env:                         # Custom environment variables
    API_KEY: "secret-key"

# Auto-Scaling Configuration
scaling:
  min_workers: 1          # Minimum number of instances
  max_workers: 5          # Maximum number of instances
  queue_threshold: 10     # Request queue depth that triggers scale-up
  scale_down_delay: 60    # Seconds of idle time before scaling down

# Timeouts
timeouts:
  read_timeout_seconds: 30
  write_timeout_seconds: 30
  idle_timeout_seconds: 120
```

## Auto-Scaling

TQServer features a built-in load balancer and auto-scaler for Bun workers.

- **Load Balancing**: Requests are distributed across available worker instances using a Round-Robin strategy.
- **Queueing**: If all workers are busy, requests are queued.
- **Scale Up**: If the queue depth exceeds `queue_threshold`, new worker instances are spawned (up to `max_workers`).
- **Scale Down**: Idle workers are automatically terminated after `scale_down_delay` seconds to save resources.

## Development Workflow

1. **Edit Code**: Modify your `.ts` files.
2. **Auto-Reload**: TQServer detects the change, rebuilds (installs dependencies if needed), and performs a rolling restart of your worker instances.
3. **Debug**: Check `server/logs/worker_NAME_PORT.log` for output.

## Best Practices

1. **State Management**: Since workers can be scaled horizontally, **do not store state in memory** (global variables) if you expect it to persist or be shared across requests. Use an external database (SQLite, Postgres, Redis) for state.
2. **Fast Startup**: Keep your initialization logic minimal to allow fast scale-up.
3. **Graceful Shutdown**: Handle `SIGINT` / `SIGTERM` if you have cleanup tasks (Express handles this well by default for connection closing).
4. **Environment Variables**: Access configuration via `process.env`. TQServer injects `WORKER_PORT`, `WORKER_NAME`, `WORKER_ROUTE`, etc.
