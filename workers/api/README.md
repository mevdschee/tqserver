# Kotlin API Worker

A TQServer worker written in Kotlin that provides an in-memory CRUD REST API service.

## Overview

This worker demonstrates how to build TQServer workers using Kotlin and the Ktor framework. It implements a complete REST API for managing items with full CRUD operations.

## Features

- ✅ **RESTful API**: Standard HTTP methods (GET, POST, PUT, DELETE)
- ✅ **In-Memory Storage**: Thread-safe ConcurrentHashMap for data storage
- ✅ **JSON Serialization**: Automatic JSON parsing and generation
- ✅ **Validation**: Input validation with clear error messages
- ✅ **Health Checks**: Standard `/health` endpoint for TQServer
- ✅ **Logging**: Request logging with timestamps
- ✅ **TQServer Integration**: Reads configuration from environment variables

## Prerequisites

- Java 17 or higher
- Gradle 7.x or higher (wrapper included)
- TQServer running instance

## Building

### Using Gradle Wrapper (Recommended)

```bash
cd workers/api
./gradlew build
```

### Using System Gradle

```bash
cd workers/api
gradle build
```

The build will create a fat JAR at `build/libs/api.jar` containing all dependencies.

## Running Manually (Without TQServer)

For testing purposes, you can run the worker directly:

```bash
# Set environment variables
export WORKER_PORT=9000
export WORKER_ROUTE=/api
export WORKER_MODE=dev

# Run the JAR
java -jar build/libs/api.jar
```

## Running via TQServer

TQServer will automatically:
1. Detect the worker in the `workers/api` directory
2. Build the worker using Gradle (if not already built)
3. Start the worker on an assigned port
4. Route requests from `/api/*` to this worker
5. Rebuild and restart on source code changes (hot-reload)

## API Endpoints

### Root
- **GET /api/** - Service information and available endpoints

### Items CRUD

- **GET /api/items** - List all items
- **GET /api/items/:id** - Get a specific item by ID
- **POST /api/items** - Create a new item
- **PUT /api/items/:id** - Update an existing item
- **DELETE /api/items/:id** - Delete an item

### Utility

- **GET /api/health** - Health check (returns "OK")
- **GET /api/stats** - Service statistics (item count, config)

## Example Usage

### Create an Item

```bash
curl -X POST http://localhost:3000/api/items \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Example Item",
    "description": "This is a test item"
  }'
```

Response:
```json
{
  "id": 1,
  "name": "Example Item",
  "description": "This is a test item",
  "createdAt": "2026-01-07T10:30:00",
  "updatedAt": "2026-01-07T10:30:00"
}
```

### List All Items

```bash
curl http://localhost:3000/api/items
```

Response:
```json
[
  {
    "id": 1,
    "name": "Example Item",
    "description": "This is a test item",
    "createdAt": "2026-01-07T10:30:00",
    "updatedAt": "2026-01-07T10:30:00"
  }
]
```

### Get Single Item

```bash
curl http://localhost:3000/api/items/1
```

### Update Item

```bash
curl -X PUT http://localhost:3000/api/items/1 \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Updated Item",
    "description": "Updated description"
  }'
```

### Delete Item

```bash
curl -X DELETE http://localhost:3000/api/items/1
```

Response:
```json
{
  "message": "Item deleted successfully"
}
```

### Get Stats

```bash
curl http://localhost:3000/api/stats
```

Response:
```json
{
  "totalItems": 5,
  "workerPort": 10000,
  "workerRoute": "/api",
  "mode": "dev"
}
```

## Data Model

### Item

```kotlin
data class Item(
    val id: Long,
    val name: String,
    val description: String,
    val createdAt: String,
    val updatedAt: String
)
```

### Create Item Request

```kotlin
data class CreateItemRequest(
    val name: String,
    val description: String
)
```

### Update Item Request

```kotlin
data class UpdateItemRequest(
    val name: String? = null,
    val description: String? = null
)
```

## Error Handling

The API returns appropriate HTTP status codes and JSON error responses:

### 400 Bad Request
```json
{
  "error": "validation_error",
  "message": "Name cannot be empty"
}
```

### 404 Not Found
```json
{
  "error": "not_found",
  "message": "Item with ID 999 not found"
}
```

## Configuration

Edit `config/worker.yaml` to adjust:

- **path**: URL path prefix (default: `/api`)
- **type**: Worker type (future feature, set to `kotlin`)
- **runtime.go_mem_limit**: Memory limit for the JVM
- **timeouts**: HTTP timeout settings

## Development

### Project Structure

```
workers/api/
├── build.gradle.kts          # Gradle build configuration
├── settings.gradle.kts       # Gradle settings
├── bin/
│   └── api                   # Wrapper script (executed by TQServer)
├── build/
│   └── libs/
│       └── api.jar           # Compiled JAR (generated)
├── config/
│   └── worker.yaml           # Worker configuration
└── src/
    └── main/
        └── kotlin/
            └── Main.kt       # Main application code
```

### Adding Dependencies

Edit `build.gradle.kts` and add dependencies in the `dependencies` block:

```kotlin
dependencies {
    implementation("group:artifact:version")
}
```

Then rebuild:

```bash
./gradlew build
```

### Hot Reload

When running via TQServer in development mode:
1. Edit any `.kt` file in `src/main/kotlin/`
2. Save the file
3. TQServer detects the change (future feature)
4. Rebuilds the worker automatically
5. Restarts the worker with zero-downtime

## Testing

Run tests using Gradle:

```bash
./gradlew test
```

## Performance Considerations

### Memory
- Default JVM heap: 256MB-512MB
- Adjust via `JAVA_OPTS` environment variable
- Set `go_mem_limit` in `config/worker.yaml`

### Startup Time
- Cold start: ~2-3 seconds
- Includes JVM initialization and Ktor server startup

### Thread Safety
- Uses `ConcurrentHashMap` for thread-safe in-memory storage
- `AtomicLong` for thread-safe ID generation
- Safe for concurrent requests

## Troubleshooting

### Worker Won't Start

1. **Check JAR exists**: `ls -la build/libs/api.jar`
2. **Verify Java version**: `java -version` (should be 17+)
3. **Check logs**: Look in TQServer logs for build errors
4. **Manual build**: `cd workers/api && ./gradlew build --info`

### Build Failures

1. **Clean and rebuild**: `./gradlew clean build`
2. **Check Gradle**: `./gradlew --version`
3. **Check dependencies**: `./gradlew dependencies`

### Port Conflicts

TQServer assigns ports automatically. If you run the worker manually, ensure the port is available:

```bash
lsof -i :9000
```

## Future Enhancements

- [ ] Persistent storage (database integration)
- [ ] Authentication and authorization
- [ ] Pagination for large item lists
- [ ] Search and filtering
- [ ] API versioning
- [ ] OpenAPI/Swagger documentation
- [ ] Metrics and monitoring
- [ ] Caching layer

## License

Same as TQServer project.

## Contributing

1. Follow Kotlin coding conventions
2. Add tests for new features
3. Update this README with changes
4. Test hot-reload functionality

## Resources

- [Ktor Documentation](https://ktor.io/docs/)
- [Kotlin Documentation](https://kotlinlang.org/docs/)
- [TQServer Documentation](../../docs/README.md)
