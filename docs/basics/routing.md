# Routing

- [Introduction](#introduction)
- [Filesystem-Based Routing](#filesystem-based-routing)
- [Basic Routing](#basic-routing)
- [Route Parameters](#route-parameters)
- [HTTP Methods](#http-methods)
- [Route Groups](#route-groups)
- [Fallback Routes](#fallback-routes)
- [URL Generation](#url-generation)

## Introduction

TQServer uses filesystem-based routing where your URL structure mirrors your workers directory structure. This intuitive approach makes it easy to understand and organize your application's routes.

## Filesystem-Based Routing

Routes are automatically determined by your workers directory structure:

```
workers/
├── index/          → http://localhost:8080/
├── api/            → http://localhost:8080/api/*
├── admin/          → http://localhost:8080/admin/*
└── blog/           → http://localhost:8080/blog/*
```

### Routing Rules

1. **Index Worker**: The `index` worker handles the root path `/`
2. **Named Workers**: Other workers handle paths matching their directory name
3. **Wildcard**: Workers handle all sub-paths under their base path

### Examples

```
GET /                    → workers/index/
GET /about               → workers/index/ (with path="/about")
GET /api/users           → workers/api/ (with path="/users")
GET /api/users/123       → workers/api/ (with path="/users/123")
GET /admin/dashboard     → workers/admin/ (with path="/dashboard")
```

## Basic Routing

Inside a worker, you define routes using Go's standard HTTP handlers:

```go
// workers/api/src/main.go
package main

import (
    "encoding/json"
    "net/http"
)

func main() {
    // Define routes
    http.HandleFunc("/", homeHandler)
    http.HandleFunc("/users", usersHandler)
    http.HandleFunc("/users/", userDetailHandler)
    
    // Start server
    port := os.Getenv("WORKER_PORT")
    http.ListenAndServe(":" + port, nil)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
    json.NewEncoder(w).Encode(map[string]string{
        "message": "API Home",
    })
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
    // Handle /api/users
    users := []string{"Alice", "Bob", "Charlie"}
    json.NewEncoder(w).Encode(users)
}

func userDetailHandler(w http.ResponseWriter, r *http.Request) {
    // Handle /api/users/123
    // Extract ID from path
    path := strings.TrimPrefix(r.URL.Path, "/users/")
    json.NewEncoder(w).Encode(map[string]string{
        "id": path,
    })
}
```

### Route Matching

Routes are matched in order:
1. **Exact match**: `/users` matches exactly `/users`
2. **Prefix match**: `/users/` matches `/users/123`, `/users/abc`, etc.

## Route Parameters

Extract parameters from URLs:

### Path Parameters

```go
// GET /api/users/123
func userDetailHandler(w http.ResponseWriter, r *http.Request) {
    // Extract ID from path
    parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
    userID := parts[len(parts)-1]
    
    // Use userID
    user := GetUser(userID)
    json.NewEncoder(w).Encode(user)
}
```

### Query Parameters

```go
// GET /api/users?role=admin&status=active
func usersHandler(w http.ResponseWriter, r *http.Request) {
    // Parse query parameters
    role := r.URL.Query().Get("role")
    status := r.URL.Query().Get("status")
    
    // Filter users
    users := FilterUsers(role, status)
    json.NewEncoder(w).Encode(users)
}
```

### Using a Router Library

For more sophisticated routing, use a router library like `gorilla/mux`:

```go
import "github.com/gorilla/mux"

func main() {
    r := mux.NewRouter()
    
    // Route with path parameters
    r.HandleFunc("/users/{id}", userHandler)
    r.HandleFunc("/posts/{year:[0-9]+}/{month:[0-9]+}", postsHandler)
    
    port := os.Getenv("WORKER_PORT")
    http.ListenAndServe(":" + port, r)
}

func userHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    userID := vars["id"]
    
    user := GetUser(userID)
    json.NewEncoder(w).Encode(user)
}
```

## HTTP Methods

Handle different HTTP methods:

### Standard Handler

```go
func userHandler(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        // GET /users - List users
        users := ListUsers()
        json.NewEncoder(w).Encode(users)
        
    case http.MethodPost:
        // POST /users - Create user
        var user User
        json.NewDecoder(r.Body).Decode(&user)
        created := CreateUser(user)
        json.NewEncoder(w).Encode(created)
        
    case http.MethodPut:
        // PUT /users - Update user
        var user User
        json.NewDecoder(r.Body).Decode(&user)
        updated := UpdateUser(user)
        json.NewEncoder(w).Encode(updated)
        
    case http.MethodDelete:
        // DELETE /users - Delete user
        id := r.URL.Query().Get("id")
        DeleteUser(id)
        w.WriteHeader(http.StatusNoContent)
        
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}
```

### Using gorilla/mux

```go
r := mux.NewRouter()

// Method-specific routes
r.HandleFunc("/users", listUsers).Methods("GET")
r.HandleFunc("/users", createUser).Methods("POST")
r.HandleFunc("/users/{id}", getUser).Methods("GET")
r.HandleFunc("/users/{id}", updateUser).Methods("PUT")
r.HandleFunc("/users/{id}", deleteUser).Methods("DELETE")
```

## Route Groups

Organize related routes:

### Manual Grouping

```go
func main() {
    // API routes
    http.HandleFunc("/api/users", usersHandler)
    http.HandleFunc("/api/posts", postsHandler)
    http.HandleFunc("/api/comments", commentsHandler)
    
    // Admin routes
    http.HandleFunc("/admin/dashboard", dashboardHandler)
    http.HandleFunc("/admin/users", adminUsersHandler)
    http.HandleFunc("/admin/settings", settingsHandler)
    
    port := os.Getenv("WORKER_PORT")
    http.ListenAndServe(":" + port, nil)
}
```

### Using Subrouters

```go
import "github.com/gorilla/mux"

func main() {
    r := mux.NewRouter()
    
    // API subrouter
    api := r.PathPrefix("/api").Subrouter()
    api.HandleFunc("/users", usersHandler)
    api.HandleFunc("/posts", postsHandler)
    
    // Admin subrouter (with middleware)
    admin := r.PathPrefix("/admin").Subrouter()
    admin.Use(authMiddleware)
    admin.HandleFunc("/dashboard", dashboardHandler)
    admin.HandleFunc("/users", adminUsersHandler)
    
    port := os.Getenv("WORKER_PORT")
    http.ListenAndServe(":" + port, r)
}
```

### Separate Workers

For larger applications, create separate workers:

```
workers/
├── api/            # API routes
│   └── src/
│       └── main.go
├── admin/          # Admin routes
│   └── src/
│       └── main.go
└── web/            # Public website
    └── src/
        └── main.go
```

## Fallback Routes

Handle 404 and catch-all routes:

### 404 Handler

```go
func main() {
    http.HandleFunc("/", rootHandler)
    http.HandleFunc("/users", usersHandler)
    
    // Fallback for undefined routes
    http.HandleFunc("/", notFoundHandler)
    
    port := os.Getenv("WORKER_PORT")
    http.ListenAndServe(":" + port, nil)
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusNotFound)
    json.NewEncoder(w).Encode(map[string]string{
        "error": "Route not found",
        "path": r.URL.Path,
    })
}
```

### Catch-All with gorilla/mux

```go
r := mux.NewRouter()

// Define specific routes
r.HandleFunc("/users", usersHandler)
r.HandleFunc("/posts", postsHandler)

// Catch-all route (must be last)
r.NotFoundHandler = http.HandlerFunc(notFoundHandler)
```

## URL Generation

Generate URLs dynamically:

### Basic URLs

```go
// Relative URLs
url := "/users/123"

// Absolute URLs
baseURL := "http://localhost:8080"
url := fmt.Sprintf("%s/api/users/%s", baseURL, userID)
```

### Query Parameters

```go
// Build query string
params := url.Values{}
params.Add("role", "admin")
params.Add("status", "active")

url := "/users?" + params.Encode()
// Result: /users?role=admin&status=active
```

### URL Helper Functions

```go
// helpers.go
func RouteURL(path string) string {
    baseURL := os.Getenv("BASE_URL")
    return baseURL + path
}

func UserURL(userID string) string {
    return RouteURL("/users/" + userID)
}

func PostURL(postID string) string {
    return RouteURL("/posts/" + postID)
}

// Usage
url := UserURL("123")  // http://localhost:8080/api/users/123
```

## Advanced Routing Patterns

### RESTful API

```go
r := mux.NewRouter()

// Users resource
users := r.PathPrefix("/users").Subrouter()
users.HandleFunc("", listUsers).Methods("GET")
users.HandleFunc("", createUser).Methods("POST")
users.HandleFunc("/{id}", getUser).Methods("GET")
users.HandleFunc("/{id}", updateUser).Methods("PUT")
users.HandleFunc("/{id}", deleteUser).Methods("DELETE")

// Posts resource (nested)
r.HandleFunc("/users/{userId}/posts", userPostsHandler).Methods("GET")
r.HandleFunc("/users/{userId}/posts", createUserPostHandler).Methods("POST")
```

### Version Prefixes

```go
// API versioning
v1 := r.PathPrefix("/v1").Subrouter()
v1.HandleFunc("/users", v1UsersHandler)

v2 := r.PathPrefix("/v2").Subrouter()
v2.HandleFunc("/users", v2UsersHandler)
```

### Subdomain Routing

TQServer routes are based on paths, but you can handle subdomains within workers:

```go
func handler(w http.ResponseWriter, r *http.Request) {
    host := r.Host
    
    switch {
    case strings.HasPrefix(host, "api."):
        handleAPIRequest(w, r)
    case strings.HasPrefix(host, "admin."):
        handleAdminRequest(w, r)
    default:
        handleWebRequest(w, r)
    }
}
```

## Best Practices

1. **Use separate workers** for different application sections (API, admin, web)
2. **Keep routes organized** with clear naming conventions
3. **Use router libraries** for complex routing needs
4. **Validate parameters** before using them
5. **Return appropriate status codes** (200, 201, 404, 500, etc.)
6. **Document routes** in your worker's README

## Next Steps

- [Middleware](middleware.md) - Add middleware to routes
- [Controllers](controllers.md) - Organize handler logic
- [Requests](requests.md) - Parse and validate requests
- [Responses](responses.md) - Format and send responses
- [URL Generation](urls.md) - Advanced URL building
