# Kotlin Worker Support Plan

## Overview

This document outlines the plan for adding Kotlin-based worker support to TQServer, enabling developers to build workers using Kotlin alongside the existing Go-based workers.

## Goals

1. Support Kotlin JVM workers that can handle HTTP requests
2. Maintain the same worker lifecycle (build, start, reload) as Go workers
3. Provide seamless integration with existing TQServer infrastructure
4. Enable hot-reload for Kotlin workers during development
5. Minimize changes to existing architecture

## Architecture Design

### Worker Types

TQServer will support multiple worker types based on a `type` field in `config/worker.yaml`:

```yaml
path: "/api"
type: "kotlin"  # Options: "go" (default), "kotlin", "python", "node"
runtime:
  # ... existing runtime config
```

### Kotlin Worker Structure

```
workers/
  api/
    src/
      main/
        kotlin/
          Main.kt
      resources/
        application.conf
    build/
      libs/
        api.jar
    config/
      worker.yaml
    bin/
      api  (wrapper script)
    public/
    views/
```

### Build Process

#### 1. Kotlin Build Support

The builder package (`pkg/builder/builder.go`) will be extended to:

- Detect worker type from `config/worker.yaml`
- Use appropriate build commands based on type:
  - **Go**: `go build -o bin/{worker} src`
  - **Kotlin**: `gradle build` or `kotlinc` with classpath management
  
#### 2. Gradle Integration

For Kotlin workers, we'll use Gradle for dependency management and building:

```groovy
// build.gradle.kts
plugins {
    kotlin("jvm") version "1.9.22"
    application
}

application {
    mainClass.set("MainKt")
}

dependencies {
    implementation("io.ktor:ktor-server-core:2.3.7")
    implementation("io.ktor:ktor-server-netty:2.3.7")
    implementation("io.ktor:ktor-server-content-negotiation:2.3.7")
    implementation("io.ktor:ktor-serialization-kotlinx-json:2.3.7")
}

tasks.jar {
    manifest {
        attributes["Main-Class"] = "MainKt"
    }
    duplicatesStrategy = DuplicatesStrategy.EXCLUDE
    from(configurations.runtimeClasspath.get().map { if (it.isDirectory) it else zipTree(it) })
}
```

#### 3. Runtime Wrapper

Create a shell script wrapper (`bin/{worker}`) that:
- Sets up Java classpath
- Passes environment variables (WORKER_PORT, WORKER_ROUTE, etc.)
- Executes the JAR file
- Handles graceful shutdown

```bash
#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKER_DIR="$(dirname "$SCRIPT_DIR")"
JAR_FILE="$WORKER_DIR/build/libs/$(basename "$WORKER_DIR").jar"

exec java -jar "$JAR_FILE" "$@"
```

### Worker Runtime

#### Environment Variables

Kotlin workers will receive the same environment variables as Go workers:

- `WORKER_PORT`: Assigned port number
- `WORKER_ROUTE`: URL path prefix
- `WORKER_MODE`: "dev" or "prod"
- `WORKER_READ_TIMEOUT_SECONDS`
- `WORKER_WRITE_TIMEOUT_SECONDS`
- `WORKER_IDLE_TIMEOUT_SECONDS`

#### Kotlin Runtime Helper Library

Create a companion library (similar to `pkg/worker/runtime.go`) that Kotlin workers can import:

```kotlin
// tqserver-kotlin-runtime/src/main/kotlin/Runtime.kt
package com.tqserver.runtime

import io.ktor.server.engine.*
import io.ktor.server.netty.*

class WorkerRuntime {
    val port: Int = System.getenv("WORKER_PORT")?.toIntOrNull() ?: 9000
    val route: String = System.getenv("WORKER_ROUTE") ?: "/"
    val mode: String = System.getenv("WORKER_MODE") ?: "dev"
    val readTimeout: Long = System.getenv("WORKER_READ_TIMEOUT_SECONDS")?.toLongOrNull() ?: 30
    val writeTimeout: Long = System.getenv("WORKER_WRITE_TIMEOUT_SECONDS")?.toLongOrNull() ?: 30
    val idleTimeout: Long = System.getenv("WORKER_IDLE_TIMEOUT_SECONDS")?.toLongOrNull() ?: 120
    
    fun isDevelopmentMode(): Boolean = mode == "dev"
    
    fun createServer(configure: Application.() -> Unit) {
        embeddedServer(Netty, port = port) {
            configure()
        }.start(wait = true)
    }
}
```

### File Watching & Hot Reload

The file watcher (`pkg/watcher/filewatcher.go`) will:

1. Watch `src/**/*.kt` files for Kotlin workers
2. Trigger rebuild on change
3. Use the same restart mechanism as Go workers

## Implementation Plan

### Phase 1: Core Infrastructure (Week 1-2)

#### Task 1.1: Update Configuration Schema
- [ ] Add `type` field to worker configuration
- [ ] Update config parsing in `server/src/config.go`
- [ ] Add validation for supported worker types

#### Task 1.2: Extend Builder
- [ ] Modify `pkg/builder/builder.go` to detect worker type
- [ ] Add `BuildKotlinWorker()` method
- [ ] Implement Gradle build integration
- [ ] Generate wrapper script for Kotlin workers

#### Task 1.3: Update File Watcher
- [ ] Extend `pkg/watcher/filewatcher.go` to watch `.kt` files
- [ ] Add pattern matching for Kotlin source files
- [ ] Update rebuild triggers

#### Task 1.4: Runtime Library
- [ ] Create `kotlin-runtime/` directory
- [ ] Implement `WorkerRuntime.kt` helper class
- [ ] Setup Gradle project for runtime library
- [ ] Publish to local Maven or include as dependency

### Phase 2: Example Worker (Week 2)

#### Task 2.1: Create API Worker Structure
- [ ] Create `workers/api/` directory structure
- [ ] Setup `build.gradle.kts` with dependencies
- [ ] Create `config/worker.yaml` with type "kotlin"

#### Task 2.2: Implement CRUD Service
- [ ] Design in-memory data store (thread-safe)
- [ ] Implement REST endpoints:
  - `GET /api/items` - List all items
  - `GET /api/items/:id` - Get item by ID
  - `POST /api/items` - Create new item
  - `PUT /api/items/:id` - Update item
  - `DELETE /api/items/:id` - Delete item
- [ ] Add JSON serialization/deserialization
- [ ] Implement health check endpoint

#### Task 2.3: Integration Testing
- [ ] Test worker startup via TQServer
- [ ] Verify routing and proxying
- [ ] Test hot-reload functionality
- [ ] Test graceful shutdown

### Phase 3: Documentation (Week 3)

#### Task 3.1: User Documentation
- [ ] Update `docs/workers/creating.md` with Kotlin section
- [ ] Create `docs/workers/kotlin.md` with detailed guide
- [ ] Add example code snippets
- [ ] Document dependency management

#### Task 3.2: Developer Documentation
- [ ] Document builder extension points
- [ ] Add architecture diagrams
- [ ] Document testing procedures
- [ ] Update README.md

### Phase 4: Testing & Refinement (Week 3-4)

#### Task 4.1: Automated Testing
- [ ] Add unit tests for Kotlin builder
- [ ] Add integration tests for Kotlin workers
- [ ] Test error handling and recovery
- [ ] Performance testing

#### Task 4.2: Error Handling
- [ ] Improve build error reporting
- [ ] Add build error page for Kotlin compilation errors
- [ ] Handle missing Gradle/Kotlin dependencies
- [ ] Graceful degradation

## Technical Considerations

### Dependencies

#### Server-side Requirements
- Gradle (for building Kotlin workers)
- Kotlin compiler (included with Gradle)
- Java Runtime Environment (JRE 17+)

#### Developer Requirements
- Kotlin knowledge
- Understanding of Ktor or similar frameworks
- Gradle familiarity

### Performance Considerations

1. **Startup Time**: JVM startup is slower than Go binaries
   - Mitigation: Use GraalVM native image for production (optional)
   - Keep development mode with standard JVM for faster iteration

2. **Memory Usage**: JVM workers consume more memory
   - Mitigation: Configure heap size via `JAVA_OPTS`
   - Document recommended memory limits

3. **Build Time**: Gradle builds may be slower
   - Mitigation: Use Gradle daemon
   - Implement incremental compilation
   - Cache dependencies

### Security Considerations

1. **Dependency Management**: Use Gradle's dependency verification
2. **JVM Security**: Run with appropriate security manager policies
3. **Isolation**: Use the same process isolation as Go workers

## Future Enhancements

### Short-term (3-6 months)
- Support for Python workers (Django/Flask)
- Support for Node.js workers (Express/Fastify)
- Worker type templates and scaffolding
- Improved build caching

### Long-term (6-12 months)
- GraalVM native image support for Kotlin workers
- Multi-language worker communication (message bus)
- Shared state management across workers
- Plugin system for custom worker types

## Success Metrics

1. **Developer Experience**
   - Time to create first Kotlin worker < 5 minutes
   - Hot-reload works consistently
   - Clear error messages

2. **Performance**
   - Build time < 10 seconds for small workers
   - Startup time < 3 seconds in development
   - No performance degradation in Go workers

3. **Reliability**
   - Workers restart reliably after crashes
   - Build failures are reported clearly
   - No memory leaks over 24 hours

## Migration Path

For teams wanting to migrate existing services:

1. Create new Kotlin worker directory structure
2. Copy business logic to Kotlin
3. Configure worker.yaml with appropriate path
4. Test alongside existing Go workers
5. Gradually route traffic to Kotlin worker
6. Decommission old worker when ready

## Risks and Mitigations

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| JVM startup overhead affects hot-reload | High | High | Use Gradle daemon, implement smart rebuild detection |
| Increased complexity in builder | Medium | High | Well-tested abstractions, clear interfaces |
| Dependency conflicts between workers | Medium | Low | Isolated worker processes, separate classpaths |
| Build tool unavailability | High | Low | Check for tools at startup, clear error messages |

## Conclusion

This plan provides a structured approach to adding Kotlin worker support to TQServer. The phased implementation allows for iterative development and testing while maintaining the stability of existing Go-based workers. The architecture is designed to be extensible for future language support.
