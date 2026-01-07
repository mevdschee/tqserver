# Kotlin Worker Quick Reference

## Build & Run

```bash
# Build worker
cd workers/api
./gradlew build

# Run via TQServer
cd ../..
bash start.sh

# Run standalone (testing)
export WORKER_PORT=9000 WORKER_ROUTE=/api WORKER_MODE=dev
./workers/api/bin/api
```

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/` | Service info |
| GET | `/api/health` | Health check |
| GET | `/api/items` | List all items |
| GET | `/api/items/:id` | Get item by ID |
| POST | `/api/items` | Create item |
| PUT | `/api/items/:id` | Update item |
| DELETE | `/api/items/:id` | Delete item |
| GET | `/api/stats` | Statistics |

## Example Requests

```bash
# Create item
curl -X POST http://localhost:3000/api/items \
  -H "Content-Type: application/json" \
  -d '{"name":"Laptop","description":"Dell XPS"}'

# List all
curl http://localhost:3000/api/items

# Get one
curl http://localhost:3000/api/items/1

# Update
curl -X PUT http://localhost:3000/api/items/1 \
  -H "Content-Type: application/json" \
  -d '{"name":"Gaming Laptop"}'

# Delete
curl -X DELETE http://localhost:3000/api/items/1

# Stats
curl http://localhost:3000/api/stats
```

## Project Structure

```
workers/api/
├── src/main/kotlin/Main.kt    # Application code
├── build.gradle.kts            # Build config
├── config/worker.yaml          # Worker config
├── bin/api                     # Wrapper script
└── build/libs/api.jar          # Built JAR
```

## Configuration

**config/worker.yaml:**
```yaml
path: "/api"           # URL path
type: "kotlin"         # Worker type
runtime:
  go_mem_limit: "1GiB" # Memory limit
timeouts:
  read_timeout_seconds: 30
  write_timeout_seconds: 30
```

## Environment Variables

Set by TQServer:
- `WORKER_PORT` - Assigned port
- `WORKER_ROUTE` - Path prefix  
- `WORKER_MODE` - dev/prod
- `WORKER_*_TIMEOUT_SECONDS` - Timeouts

## Common Commands

```bash
# Clean build
./gradlew clean build

# Build without tests
./gradlew build -x test

# Run demo
./demo.sh

# Check dependencies
./gradlew dependencies

# Upgrade Gradle wrapper
gradle wrapper --gradle-version 8.5
```

## Troubleshooting

```bash
# JAR not found
ls -la build/libs/api.jar
./gradlew build

# Port in use
lsof -i :10000
kill -9 <PID>

# Memory issues
export JAVA_OPTS="-Xmx1g -Xms512m"

# Dependencies issue
./gradlew build --refresh-dependencies
```

## Development Flow

1. Edit `src/main/kotlin/Main.kt`
2. Build: `./gradlew build`
3. Restart TQServer: `bash start.sh`
4. Test: `curl http://localhost:3000/api/`

## Key Classes

```kotlin
// Runtime helper
class WorkerRuntime {
    val port: Int
    val route: String
    val mode: String
}

// Data model
@Serializable
data class Item(
    val id: Long,
    val name: String,
    val description: String,
    val createdAt: String,
    val updatedAt: String
)

// Service
class ItemService {
    fun create(request: CreateItemRequest): Item
    fun getAll(): List<Item>
    fun getById(id: Long): Item?
    fun update(id: Long, request: UpdateItemRequest): Item?
    fun delete(id: Long): Boolean
}
```

## Documentation

- Full Guide: [docs/workers/kotlin.md](../workers/kotlin.md)
- Implementation Plan: [docs/KOTLIN_WORKER_SUPPORT.md](KOTLIN_WORKER_SUPPORT.md)
- Example README: [workers/api/README.md](../../workers/api/README.md)

## Dependencies

- Java 17+
- Gradle 7.x+
- Ktor 2.3.7
- Kotlin 1.9.22

## Tips

✅ Always implement `/health` endpoint  
✅ Use thread-safe collections (ConcurrentHashMap)  
✅ Handle errors gracefully  
✅ Log all requests  
✅ Validate input  
✅ Return proper HTTP status codes  
✅ Build fat JAR with all dependencies  

## Future Features

- Automatic hot-reload
- Build error pages
- Native image support
- Worker templates
