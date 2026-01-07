# Development Mode Error Pages

## Overview

In development mode, when a worker fails to compile, TQServer will serve an HTML error page showing the compilation errors instead of trying to use the old executable. This makes it easier to see and fix compilation errors during development.

## Features

- **Automatic Error Detection**: When a worker fails to build, the error is captured and stored
- **HTML Error Page**: A clean, readable error page shows the compilation output
- **Auto-Refresh**: The error page automatically refreshes every 2 seconds to check if the build is fixed
- **Development Mode Only**: This feature only activates in development mode to avoid exposing errors in production

## How It Works

1. When file changes are detected, TQServer attempts to rebuild the worker
2. If the build fails:
   - In **dev mode**: The error is stored and an HTML error page is served for that worker's routes
   - In **prod mode**: The error is returned and the old binary continues to run (if available)
3. When you fix the error and save, the build succeeds and the worker restarts normally
4. The error page auto-refreshes, so you'll see your working application as soon as the build succeeds

## Enabling Development Mode

Set the mode when starting the server:

```bash
# Via command line flag
./server/bin/tqserver --mode dev

# Via environment variable
export TQSERVER_MODE=dev
./server/bin/tqserver

# Mode defaults to 'dev' if not specified
./server/bin/tqserver
```

## Example Error Page

When you have a compilation error, you'll see a page like this:

```
⚠️ Compilation Error
Worker: index

[Error output from Go compiler showing the syntax error]

ℹ️ Development Mode
The compilation failed. Fix the errors in your code and save the file.
The page will automatically reload once the build succeeds.

💡 This error page is only shown in development mode.
```

## Testing the Feature

You can test this by temporarily introducing a syntax error in a worker:

1. Start the server in dev mode:
   ```bash
   ./server/bin/tqserver --mode dev
   ```

2. Edit a worker file (e.g., `workers/index/src/main.go`) and introduce a syntax error:
   ```go
   // Add this line somewhere in the file
   this is not valid go code
   ```

3. Save the file - the watcher will detect the change and attempt to rebuild

4. Visit the worker's route in your browser (e.g., `http://localhost:8080/`)

5. You should see the HTML error page with the compilation error

6. Fix the error and save - within 2 seconds, the page will refresh and show your working application

## Configuration

The mode is controlled by:

1. Command line flag: `--mode dev` or `--mode prod`
2. Environment variable: `TQSERVER_MODE=dev` or `TQSERVER_MODE=prod`
3. Default: `dev` (if neither flag nor env var is set)

## Production Behavior

In production mode (`--mode prod` or `TQSERVER_MODE=prod`):

- Build errors cause the worker to fail to start
- The previous working binary continues to run (if available)
- Error pages are NOT shown to users
- Build errors are logged but not exposed

This ensures that compilation errors never leak to production users.

## Technical Details

### Changes Made

1. **Config**: Added `Mode` field and `IsDevelopmentMode()` method
2. **Worker**: Added `HasBuildError` and `BuildError` fields to track compilation failures
3. **Builder**: Captures the full error output from failed builds
4. **Supervisor**: In dev mode, stores build errors instead of failing completely
5. **Proxy**: 
   - Uses tqtemplate for template rendering
   - Loads error page from external template file
   - Template inherits from base.html layout
   - Checks for build errors and serves HTML error page in dev mode

### File Changes

- `server/src/config.go` - Added mode tracking
- `server/src/router.go` - Added build error fields to Worker
- `server/src/supervisor.go` - Modified buildWorker to store errors in dev mode
- `server/src/proxy.go` - Added tqtemplate support and external template loading
- `server/views/base.html` - Base template layout for server pages
- `server/views/build-error.html` - Error page template (extends base.html)
- `pkg/builder/builder.go` - Capture error output
- `server/src/main.go` - Added mode flag and logging

### Template Structure

The error page uses the tqtemplate system with template inheritance:

```
server/views/
├── base.html          # Base layout with header, footer, and blocks
└── build-error.html   # Error page that extends base.html
```

The error page template (`build-error.html`) extends `base.html` and overrides:
- `title` block - Sets the page title
- `styles` block - Adds error-specific CSS styling
- `content` block - Displays the error message and information
- `scripts` block - Adds auto-refresh JavaScript

## Related Documentation

- [Development Mode](../README.md#development-mode)
- [Hot Reload](../docs/architecture/hot-reload.md)
- [Worker Lifecycle](../docs/workers/lifecycle.md)
