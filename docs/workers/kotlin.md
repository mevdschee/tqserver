# Kotlin Workers Guide

This guide explains how to create and integrate Kotlin-based workers with TQServer.

## Table of Contents

- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Getting Started](#getting-started)
- [Project Structure](#project-structure)
- [Creating a Kotlin Worker](#creating-a-kotlin-worker)
- [Configuration](#configuration)
- [Building and Running](#building-and-running)
- [Development Workflow](#development-workflow)
- [Integration with TQServer](#integration-with-tqserver)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Introduction

TQServer supports multi-language workers. While Go is the primary language, you can also create workers using Kotlin and the Ktor framework. This guide focuses on Kotlin workers.

**Note:** As of now, automatic building and hot-reload for Kotlin workers requires manual integration with TQServer's builder. This is planned for future releases. Currently, you need to build Kotlin workers manually before starting TQServer.

## Prerequisites

### Required
- **Java 17+**: Kotlin runs on the JVM
- **Gradle 7.x+**: For dependency management and building
- **TQServer**: Latest version

### Recommended
- **IntelliJ IDEA**: Best IDE for Kotlin development
- **Kotlin Plugin**: If using VS Code or other editors

### Installation

**Ubuntu/Debian:**
```bash
sudo apt-get update
sudo apt-get install openjdk-17-jdk gradle
```

**macOS:**
```bash
brew install openjdk@17 gradle
```

**Verify Installation:**
```bash
java -version  # Should show 17 or higher
gradle -version
```

## Getting Started

### Quick Start

1. **Navigate to workers directory:**
   ```bash
   cd workers
   ```

2. **Copy the example API worker:**
   ```bash
   cp -r api myapi
   cd myapi
   ```

3. **Update configuration:**
   ```bash
   nano config/worker.yaml
   # Change the 'path' to your desired route
   ```

4. **Build the worker:**
   ```bash
   ./gradlew build
   ```

5. **Start TQServer:**
   ```bash
   cd ../..
   bash start.sh
   ```

The worker will be available at `http://localhost:3000/your-path`

## Project Structure

A Kotlin worker has the following structure:

```
workers/
  your-worker/
    ├── src/
    │   └── main/
    │       ├── kotlin/
    │       │   └── Main.kt          # Main application code
    │       └── resources/           # Configuration files
    ├── build/
    │   └── libs/
    │       └── your-worker.jar      # Built JAR (generated)
    ├── bin/
    │   └── your-worker              # Wrapper script (executable)
    ├── config/
    │   └── worker.yaml              # Worker configuration
    ├── build.gradle.kts             # Gradle build config
    ├── settings.gradle.kts          # Gradle settings
    ├── build.sh                     # Build helper script
    └── README.md                    # Documentation
```

## Creating a Kotlin Worker

### Step 1: Create Directory Structure

```bash
cd workers
mkdir -p myworker/src/main/kotlin
mkdir -p myworker/bin
mkdir -p myworker/config
cd myworker
```

### Step 2: Create Gradle Build File

Create `build.gradle.kts`:

```kotlin
plugins {
    kotlin("jvm") version "1.9.22"
    kotlin("plugin.serialization") version "1.9.22"
    application
}

group = "com.tqserver"
version = "1.0.0"

repositories {
    mavenCentral()
}

dependencies {
    // Ktor server
    implementation("io.ktor:ktor-server-core:2.3.7")
    implementation("io.ktor:ktor-server-netty:2.3.7")
    implementation("io.ktor:ktor-server-content-negotiation:2.3.7")
    implementation("io.ktor:ktor-serialization-kotlinx-json:2.3.7")
    
    // Kotlin serialization
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.6.2")
    
    // Logging
    implementation("ch.qos.logback:logback-classic:1.4.14")
}

application {
    mainClass.set("MainKt")
}

kotlin {
    jvmToolchain(17)
}

tasks.jar {
    manifest {
        attributes["Main-Class"] = "MainKt"
    }
    
    // Create fat JAR with all dependencies
    duplicatesStrategy = DuplicatesStrategy.EXCLUDE
    from(configurations.runtimeClasspath.get().map { 
        if (it.isDirectory) it else zipTree(it) 
    })
}
```

Create `settings.gradle.kts`:

```kotlin
rootProject.name = "myworker"
```

### Step 3: Create Main Application

Create `src/main/kotlin/Main.kt`:

```kotlin
import io.ktor.server.application.*
import io.ktor.server.engine.*
import io.ktor.server.netty.*
import io.ktor.server.response.*
import io.ktor.server.routing.*
import io.ktor.http.*

class WorkerRuntime {
    val port: Int = System.getenv("WORKER_PORT")?.toIntOrNull() ?: 9000
    val route: String = System.getenv("WORKER_ROUTE") ?: "/"
    val mode: String = System.getenv("WORKER_MODE") ?: "dev"
    
    fun log(message: String) {
        println("[${java.time.LocalDateTime.now()}] $message")
    }
}

fun main() {
    val runtime = WorkerRuntime()
    
    runtime.log("Starting worker on port ${runtime.port}")
    runtime.log("Route: ${runtime.route}")
    
    embeddedServer(Netty, port = runtime.port) {
        routing {
            // Health check (required)
            get("/health") {
                call.respondText("OK", ContentType.Text.Plain, HttpStatusCode.OK)
            }
            
            // Your routes here
            get("/") {
                call.respondText("Hello from Kotlin worker!")
            }
        }
    }.start(wait = true)
}
```

### Step 4: Create Wrapper Script

Create `bin/myworker`:

```bash
#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKER_DIR="$(dirname "$SCRIPT_DIR")"
WORKER_NAME="$(basename "$WORKER_DIR")"
JAR_FILE="$WORKER_DIR/build/libs/$WORKER_NAME.jar"

if [ ! -f "$JAR_FILE" ]; then
    echo "Error: JAR file not found at $JAR_FILE" >&2
    exit 1
fi

JAVA_OPTS="${JAVA_OPTS:--Xmx512m -Xms256m}"

exec java $JAVA_OPTS -jar "$JAR_FILE" "$@"
```

Make it executable:

```bash
chmod +x bin/myworker
```

### Step 5: Create Configuration

Create `config/worker.yaml`:

```yaml
# Path prefix for this worker
path: "/myroute"

# Worker type (for future support)
type: "kotlin"

# Runtime settings
runtime:
  go_max_procs: 2
  go_mem_limit: "1GiB"
  max_requests: 0

# Timeouts
timeouts:
  read_timeout_seconds: 30
  write_timeout_seconds: 30
  idle_timeout_seconds: 120

# Logging
logging:
  log_file: "logs/worker_{name}_{date}.log"
```

### Step 6: Initialize Gradle Wrapper

```bash
gradle wrapper --gradle-version 8.5
```

### Step 7: Build

```bash
./gradlew build
```

## Configuration

### Worker Configuration (`config/worker.yaml`)

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `path` | string | URL path prefix | `/api` |
| `type` | string | Worker type (future) | `kotlin` |
| `runtime.go_mem_limit` | string | Memory limit | `1GiB` |
| `timeouts.read_timeout_seconds` | int | Read timeout | `30` |
| `timeouts.write_timeout_seconds` | int | Write timeout | `30` |
| `timeouts.idle_timeout_seconds` | int | Idle timeout | `120` |

### Environment Variables

TQServer sets these environment variables for each worker:

- `WORKER_PORT`: Assigned port (e.g., `10000`)
- `WORKER_ROUTE`: Path prefix (e.g., `/api`)
- `WORKER_MODE`: Mode (`dev` or `prod`)
- `WORKER_READ_TIMEOUT_SECONDS`: Read timeout
- `WORKER_WRITE_TIMEOUT_SECONDS`: Write timeout
- `WORKER_IDLE_TIMEOUT_SECONDS`: Idle timeout

Access them in your Kotlin code:

```kotlin
val port = System.getenv("WORKER_PORT")?.toIntOrNull() ?: 9000
val route = System.getenv("WORKER_ROUTE") ?: "/"
val mode = System.getenv("WORKER_MODE") ?: "dev"
```

## Building and Running

### Building

**Development build:**
```bash
./gradlew build
```

**Clean build:**
```bash
./gradlew clean build
```

**Fast build (skip tests):**
```bash
./gradlew build -x test
```

**With detailed output:**
```bash
./gradlew build --info
```

### Running Standalone (Testing)

```bash
# Set environment
export WORKER_PORT=9000
export WORKER_ROUTE=/api
export WORKER_MODE=dev

# Run
./bin/myworker
```

### Running via TQServer

```bash
# From project root
bash start.sh
```

TQServer will:
1. Detect the worker
2. Check if `build/libs/*.jar` exists
3. Start the worker on an assigned port
4. Route requests to the worker

## Development Workflow

### 1. Edit Code

Modify `src/main/kotlin/Main.kt` or add new files.

### 2. Rebuild

```bash
./gradlew build
```

### 3. Restart TQServer

Currently, you need to restart TQServer manually after rebuilding:

```bash
# Stop (Ctrl+C)
# Start again
bash start.sh
```

**Future:** TQServer will detect `.kt` file changes and rebuild automatically.

### 4. Test

```bash
curl http://localhost:3000/your-route
```

### Hot Reload (Future)

When hot-reload is implemented, TQServer will:
1. Watch `.kt` files in `src/main/kotlin/`
2. Trigger `./gradlew build` on changes
3. Restart the worker automatically
4. Show build errors in the browser

## Integration with TQServer

### How It Works

```
User Request → TQServer (port 3000)
    ↓
Path Matching (/api/*)
    ↓
Reverse Proxy → Kotlin Worker (port 10000)
    ↓
Ktor Application Handler
    ↓
JSON Response → TQServer → User
```

### Request Flow

1. **User** makes request to `http://localhost:3000/api/items`
2. **TQServer** checks routing table, finds `/api` → worker on port `10000`
3. **TQServer** proxies request to `http://localhost:10000/items`
4. **Kotlin Worker** receives request, processes it
5. **Kotlin Worker** sends response
6. **TQServer** forwards response to user

### Health Checks

TQServer periodically checks worker health via `/health` endpoint:

```kotlin
get("/health") {
    call.respondText("OK", ContentType.Text.Plain, HttpStatusCode.OK)
}
```

**Important:** Always implement a `/health` endpoint.

## Best Practices

### 1. Error Handling

Always handle errors gracefully:

```kotlin
get("/items/{id}") {
    try {
        val id = call.parameters["id"]?.toLongOrNull()
            ?: return@get call.respond(
                HttpStatusCode.BadRequest,
                mapOf("error" to "Invalid ID")
            )
        
        val item = service.get(id)
            ?: return@get call.respond(
                HttpStatusCode.NotFound,
                mapOf("error" to "Item not found")
            )
        
        call.respond(item)
    } catch (e: Exception) {
        log.error("Error:", e)
        call.respond(
            HttpStatusCode.InternalServerError,
            mapOf("error" to "Internal server error")
        )
    }
}
```

### 2. Logging

Use structured logging:

```kotlin
fun log(level: String, message: String, extra: Map<String, Any> = emptyMap()) {
    val timestamp = java.time.LocalDateTime.now()
    val data = mapOf(
        "timestamp" to timestamp.toString(),
        "level" to level,
        "message" to message
    ) + extra
    println(kotlinx.serialization.json.Json.encodeToString(data))
}
```

### 3. Thread Safety

Use thread-safe data structures:

```kotlin
// Good
private val items = ConcurrentHashMap<Long, Item>()
private val idCounter = AtomicLong(1)

// Bad
private val items = mutableMapOf<Long, Item>()
private var idCounter = 1L
```

### 4. Resource Management

Clean up resources properly:

```kotlin
// Use 'use' for auto-closing
file.inputStream().use { stream ->
    // Read from stream
}
```

### 5. Configuration

Don't hardcode values:

```kotlin
// Good
val timeout = System.getenv("CUSTOM_TIMEOUT")?.toLongOrNull() ?: 30

// Bad
val timeout = 30
```

## Troubleshooting

### Build Failures

**Problem:** Gradle can't download dependencies

```bash
# Clear cache and retry
./gradlew build --refresh-dependencies
```

**Problem:** Kotlin version mismatch

```kotlin
// Update build.gradle.kts
kotlin("jvm") version "1.9.22"  // Use latest stable
```

### Runtime Issues

**Problem:** Worker won't start

```bash
# Check JAR exists
ls -la build/libs/*.jar

# Try running manually
export WORKER_PORT=9000
./bin/myworker
```

**Problem:** Port already in use

```bash
# Find process using port
lsof -i :10000

# Kill if necessary
kill -9 <PID>
```

**Problem:** OutOfMemoryError

```bash
# Increase heap size
export JAVA_OPTS="-Xmx1g -Xms512m"
./bin/myworker
```

### Integration Issues

**Problem:** TQServer can't find worker

- Check `config/worker.yaml` exists
- Verify `bin/myworker` is executable: `chmod +x bin/myworker`
- Check TQServer logs for errors

**Problem:** 502 Bad Gateway

- Worker might not be running
- Check worker logs
- Verify `/health` endpoint responds

**Problem:** Requests timing out

- Increase timeouts in `config/worker.yaml`
- Check for blocking operations in handlers

## Example: REST API Worker

See `workers/api/` for a complete example with:
- CRUD operations
- JSON serialization
- Error handling
- Validation
- Thread-safe in-memory storage
- Comprehensive testing

## Next Steps

- Read [workers/api/README.md](../../workers/api/README.md) for a complete example
- Explore Ktor documentation: https://ktor.io/docs/
- Learn about Kotlin coroutines for async operations
- Implement database integration
- Add authentication and authorization

## Future Features

The following features are planned for Kotlin worker support:

- **Automatic Building**: TQServer detects `.kt` changes and rebuilds
- **Hot Reload**: Zero-downtime restarts during development
- **Build Error Pages**: Show Kotlin compilation errors in browser
- **Native Images**: GraalVM support for faster startup
- **Template Support**: `gradle init` templates for quick start
- **Testing Tools**: Integrated testing framework

## Contributing

If you build something cool with Kotlin workers, consider contributing:
- Example workers
- Utility libraries
- Documentation improvements
- Bug reports and fixes

See [CONTRIBUTING.md](../prologue/contributions.md) for guidelines.
