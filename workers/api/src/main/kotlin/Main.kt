import io.ktor.server.application.*
import io.ktor.server.engine.*
import io.ktor.server.netty.*
import io.ktor.server.request.*
import io.ktor.server.response.*
import io.ktor.server.routing.*
import io.ktor.http.*
import io.ktor.serialization.kotlinx.json.*
import io.ktor.server.plugins.contentnegotiation.*
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicLong

/**
 * TQServer Worker Runtime
 * Reads configuration from environment variables set by TQServer
 */
class WorkerRuntime {
    val port: Int = System.getenv("WORKER_PORT")?.toIntOrNull() ?: 9000
    val route: String = System.getenv("WORKER_ROUTE") ?: "/"
    val mode: String = System.getenv("WORKER_MODE") ?: "dev"
    val readTimeout: Long = System.getenv("WORKER_READ_TIMEOUT_SECONDS")?.toLongOrNull() ?: 30
    val writeTimeout: Long = System.getenv("WORKER_WRITE_TIMEOUT_SECONDS")?.toLongOrNull() ?: 30
    val idleTimeout: Long = System.getenv("WORKER_IDLE_TIMEOUT_SECONDS")?.toLongOrNull() ?: 120
    
    fun isDevelopmentMode(): Boolean = mode == "dev"
    
    fun log(message: String) {
        println("[${java.time.LocalDateTime.now()}] $message")
    }
}

/**
 * Data model for CRUD items
 */
@Serializable
data class Item(
    val id: Long,
    val name: String,
    val description: String,
    val createdAt: String,
    val updatedAt: String
)

@Serializable
data class CreateItemRequest(
    val name: String,
    val description: String
)

@Serializable
data class UpdateItemRequest(
    val name: String? = null,
    val description: String? = null
)

@Serializable
data class ErrorResponse(
    val error: String,
    val message: String
)

@Serializable
data class SuccessResponse(
    val message: String,
    val data: Item? = null
)

/**
 * In-memory CRUD service (thread-safe)
 */
class ItemService {
    private val items = ConcurrentHashMap<Long, Item>()
    private val idGenerator = AtomicLong(1)
    
    fun create(request: CreateItemRequest): Item {
        val id = idGenerator.getAndIncrement()
        val now = java.time.LocalDateTime.now().toString()
        val item = Item(
            id = id,
            name = request.name,
            description = request.description,
            createdAt = now,
            updatedAt = now
        )
        items[id] = item
        return item
    }
    
    fun getAll(): List<Item> {
        return items.values.sortedBy { it.id }
    }
    
    fun getById(id: Long): Item? {
        return items[id]
    }
    
    fun update(id: Long, request: UpdateItemRequest): Item? {
        val existing = items[id] ?: return null
        val now = java.time.LocalDateTime.now().toString()
        val updated = existing.copy(
            name = request.name ?: existing.name,
            description = request.description ?: existing.description,
            updatedAt = now
        )
        items[id] = updated
        return updated
    }
    
    fun delete(id: Long): Boolean {
        return items.remove(id) != null
    }
    
    fun count(): Int = items.size
}

fun main() {
    val runtime = WorkerRuntime()
    val service = ItemService()
    
    runtime.log("Starting Kotlin API worker on port ${runtime.port}")
    runtime.log("Worker route: ${runtime.route}")
    runtime.log("Mode: ${runtime.mode}")
    
    embeddedServer(Netty, port = runtime.port) {
        install(ContentNegotiation) {
            json(Json {
                prettyPrint = true
                isLenient = true
            })
        }
        
        routing {
            // Health check endpoint (required by TQServer)
            get("/health") {
                runtime.log("GET /health")
                call.respondText("OK", ContentType.Text.Plain, HttpStatusCode.OK)
            }
            
            // List all items
            get("/items") {
                runtime.log("GET /items")
                val items = service.getAll()
                call.respond(HttpStatusCode.OK, items)
            }
            
            // Get item by ID
            get("/items/{id}") {
                val id = call.parameters["id"]?.toLongOrNull()
                runtime.log("GET /items/$id")
                
                if (id == null) {
                    call.respond(
                        HttpStatusCode.BadRequest,
                        ErrorResponse("bad_request", "Invalid ID format")
                    )
                    return@get
                }
                
                val item = service.getById(id)
                if (item == null) {
                    call.respond(
                        HttpStatusCode.NotFound,
                        ErrorResponse("not_found", "Item with ID $id not found")
                    )
                } else {
                    call.respond(HttpStatusCode.OK, item)
                }
            }
            
            // Create new item
            post("/items") {
                runtime.log("POST /items")
                try {
                    val request = call.receive<CreateItemRequest>()
                    
                    if (request.name.isBlank()) {
                        call.respond(
                            HttpStatusCode.BadRequest,
                            ErrorResponse("validation_error", "Name cannot be empty")
                        )
                        return@post
                    }
                    
                    val item = service.create(request)
                    call.respond(HttpStatusCode.Created, item)
                } catch (e: Exception) {
                    runtime.log("Error creating item: ${e.message}")
                    call.respond(
                        HttpStatusCode.BadRequest,
                        ErrorResponse("bad_request", e.message ?: "Invalid request body")
                    )
                }
            }
            
            // Update item
            put("/items/{id}") {
                val id = call.parameters["id"]?.toLongOrNull()
                runtime.log("PUT /items/$id")
                
                if (id == null) {
                    call.respond(
                        HttpStatusCode.BadRequest,
                        ErrorResponse("bad_request", "Invalid ID format")
                    )
                    return@put
                }
                
                try {
                    val request = call.receive<UpdateItemRequest>()
                    
                    if (request.name?.isBlank() == true) {
                        call.respond(
                            HttpStatusCode.BadRequest,
                            ErrorResponse("validation_error", "Name cannot be empty")
                        )
                        return@put
                    }
                    
                    val item = service.update(id, request)
                    if (item == null) {
                        call.respond(
                            HttpStatusCode.NotFound,
                            ErrorResponse("not_found", "Item with ID $id not found")
                        )
                    } else {
                        call.respond(HttpStatusCode.OK, item)
                    }
                } catch (e: Exception) {
                    runtime.log("Error updating item: ${e.message}")
                    call.respond(
                        HttpStatusCode.BadRequest,
                        ErrorResponse("bad_request", e.message ?: "Invalid request body")
                    )
                }
            }
            
            // Delete item
            delete("/items/{id}") {
                val id = call.parameters["id"]?.toLongOrNull()
                runtime.log("DELETE /items/$id")
                
                if (id == null) {
                    call.respond(
                        HttpStatusCode.BadRequest,
                        ErrorResponse("bad_request", "Invalid ID format")
                    )
                    return@delete
                }
                
                val deleted = service.delete(id)
                if (!deleted) {
                    call.respond(
                        HttpStatusCode.NotFound,
                        ErrorResponse("not_found", "Item with ID $id not found")
                    )
                } else {
                    call.respond(
                        HttpStatusCode.OK,
                        SuccessResponse("Item deleted successfully")
                    )
                }
            }
            
            // Stats endpoint
            get("/stats") {
                runtime.log("GET /stats")
                call.respond(HttpStatusCode.OK, mapOf(
                    "totalItems" to service.count(),
                    "workerPort" to runtime.port,
                    "workerRoute" to runtime.route,
                    "mode" to runtime.mode
                ))
            }
            
            // Root endpoint
            get("/") {
                runtime.log("GET /")
                call.respond(HttpStatusCode.OK, mapOf(
                    "service" to "TQServer Kotlin API Worker",
                    "version" to "1.0.0",
                    "endpoints" to listOf(
                        "GET /health",
                        "GET /items",
                        "GET /items/:id",
                        "POST /items",
                        "PUT /items/:id",
                        "DELETE /items/:id",
                        "GET /stats"
                    )
                ))
            }
        }
    }.start(wait = true)
}
