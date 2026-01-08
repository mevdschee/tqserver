# Middleware

Middleware allows you to inspect, filter, or modify HTTP requests entering your application and responses leaving it. Since TQServer workers are standard Go HTTP servers, you can use any standard Go middleware pattern.

## Basic Pattern

A middleware is a function that takes an `http.Handler` and returns a new `http.Handler`.

```go
func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Pre-processing
        log.Printf("Request: %s %s", r.Method, r.URL.Path)

        // Call the next handler
        next.ServeHTTP(w, r)

        // Post-processing (note: cannot modify response headers here if already written)
    })
}
```

## Applying Middleware

You can apply middleware to specific routes or globally.

### Global Middleware

Wrap your router (or mux) with the middleware before starting the server.

```go
func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/", homeHandler)

    // Wrap the entire mux
    handler := LoggingMiddleware(mux)
    handler = AuthMiddleware(handler) // Chaining

    runtime := worker.NewRuntime()
    runtime.StartServer(handler)
}
```

### Route-Specific Middleware

If you use a library like `gorilla/mux`, you can use `Use()` for subrouters.

```go
r := mux.NewRouter()
api := r.PathPrefix("/api").Subrouter()
api.Use(AuthMiddleware)
api.HandleFunc("/users", usersHandler)
```

## Common Middleware

### Authentication

Verify `Authorization` headers or cookies.

```go
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if token != "valid-token" {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

### JSON Content Type

Ensure all responses have the correct content type.

```go
func JSONMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        next.ServeHTTP(w, r)
    })
}
```

## Third-Party Middleware

You can use any standard Go middleware ecosystem packages, such as `rs/cors` for CORS handling or `gorilla/handlers` for logging.
