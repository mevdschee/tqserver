# Creating Workers

- [Introduction](#introduction)
- [Worker Basics](#worker-basics)
- [Creating Your First Worker](#creating-your-first-worker)
- [Worker Structure](#worker-structure)
- [Writing Handler Code](#writing-handler-code)
- [Using Templates](#using-templates)
- [Serving Static Assets](#serving-static-assets)
- [Worker Configuration](#worker-configuration)
- [Building and Running](#building-and-running)

## Introduction

Workers are the core building blocks of TQServer applications. Each worker is a self-contained Go application that handles requests for a specific section of your application.

## Worker Basics

A worker consists of:
- **Source code**: Go files in the `src/` directory
- **Binary**: Compiled executable in the `bin/` directory (auto-generated)
- **Templates**: HTML templates in the `private/` directory
- **Static assets**: CSS, JS, images in the `public/` directory
- **Configuration**: Optional `config.yaml` for worker-specific settings

### Worker Lifecycle

1. **Development**: Write code in `src/main.go`
2. **Building**: TQServer compiles to `bin/{worker-name}`
3. **Starting**: Binary starts on an assigned port
4. **Running**: Handles requests proxied from TQServer
5. **Reloading**: Auto-rebuilds and restarts on file changes

## Creating Your First Worker

Let's create a simple blog worker:

### Step 1: Create Directory Structure

```bash
mkdir -p workers/blog/{src,bin,views,config,data,public/css}
```

### Step 2: Create Main File

Create `workers/blog/src/main.go`:

```go
package main

import (
    "fmt"
    "log"
    "net/http"
    "os"
)

func main() {
    // Get port from environment (set by TQServer)
    port := os.Getenv("PORT")
    if port == "" {
        port = "9000"
    }
    
    // Define routes
    http.HandleFunc("/", homeHandler)
    http.HandleFunc("/posts", postsHandler)
    http.HandleFunc("/health", healthHandler)
    
    // Start server
    log.Printf("Blog worker starting on port %s", port)
    if err := http.ListenAndServe(":" + port, nil); err != nil {
        log.Fatalf("Server failed: %v", err)
    }
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "<h1>Welcome to the Blog</h1>")
    fmt.Fprintf(w, "<p><a href='/blog/posts'>View Posts</a></p>")
}

func postsHandler(w http.ResponseWriter, r *http.Request) {
    posts := []string{
        "Introduction to TQServer",
        "Building Fast Web Apps",
        "Go Best Practices",
    }
    
    fmt.Fprintf(w, "<h1>Blog Posts</h1><ul>")
    for _, post := range posts {
        fmt.Fprintf(w, "<li>%s</li>", post)
    }
    fmt.Fprintf(w, "</ul>")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    fmt.Fprintf(w, "OK")
}
```

### Step 3: Build and Test

TQServer will automatically detect and build your worker:

```bash
# Start TQServer (if not already running)
./bin/tqserver

# Access your worker
curl http://localhost:8080/blog/
curl http://localhost:8080/blog/posts
```

## Worker Structure

A well-organized worker follows this structure:

```
workers/blog/
├── src/
│   ├── main.go              # Entry point
│   ├── handlers.go          # HTTP handlers
│   ├── models.go            # Data structures
│   ├── services/            # Business logic
│   │   ├── posts.go
│   │   └── comments.go
│   └── middleware/          # Middleware functions
│       └── auth.go
├── private/
│   ├── views/               # HTML templates
│   │   ├── layout.html
│   │   ├── home.html
│   │   └── post.html
│   └── data/                # Data files
│       └── posts.json
├── public/
│   ├── css/
│   │   └── blog.css
│   ├── js/
│   │   └── blog.js
│   └── images/
│       └── logo.png
├── bin/                     # Compiled binary (auto-generated)
│   └── blog
└── config.yaml              # Worker configuration (optional)
```

## Writing Handler Code

### Organizing Handlers

Create `workers/blog/src/handlers.go`:

```go
package main

import (
    "encoding/json"
    "html/template"
    "net/http"
    "path/filepath"
)

type Post struct {
    ID      int    `json:"id"`
    Title   string `json:"title"`
    Content string `json:"content"`
    Author  string `json:"author"`
}

// Home page handler
func homeHandler(w http.ResponseWriter, r *http.Request) {
    tmpl := template.Must(template.ParseFiles(
        "views/layout.html",
        "views/home.html",
    ))
    
    data := map[string]interface{}{
        "Title": "Blog Home",
        "Posts": getRecentPosts(),
    }
    
    tmpl.ExecuteTemplate(w, "layout", data)
}

// List posts handler
func postsHandler(w http.ResponseWriter, r *http.Request) {
    posts := getAllPosts()
    
    // Return JSON for API requests
    if r.Header.Get("Accept") == "application/json" {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(posts)
        return
    }
    
    // Return HTML for browser requests
    tmpl := template.Must(template.ParseFiles(
        "views/layout.html",
        "views/posts.html",
    ))
    
    data := map[string]interface{}{
        "Title": "All Posts",
        "Posts": posts,
    }
    
    tmpl.ExecuteTemplate(w, "layout", data)
}

// Helper functions
func getRecentPosts() []Post {
    // In a real app, fetch from database
    return []Post{
        {ID: 1, Title: "First Post", Content: "Hello World", Author: "John"},
        {ID: 2, Title: "Second Post", Content: "More content", Author: "Jane"},
    }
}

func getAllPosts() []Post {
    return getRecentPosts()
}
```

### RESTful API Handler

Create `workers/api/src/handlers.go`:

```go
package main

import (
    "encoding/json"
    "net/http"
    "strconv"
    "strings"
)

func usersHandler(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        if strings.HasPrefix(r.URL.Path, "/users/") {
            getUserHandler(w, r)
        } else {
            listUsersHandler(w, r)
        }
    case http.MethodPost:
        createUserHandler(w, r)
    case http.MethodPut:
        updateUserHandler(w, r)
    case http.MethodDelete:
        deleteUserHandler(w, r)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

func listUsersHandler(w http.ResponseWriter, r *http.Request) {
    users := []map[string]interface{}{
        {"id": 1, "name": "Alice", "email": "alice@example.com"},
        {"id": 2, "name": "Bob", "email": "bob@example.com"},
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(users)
}

func getUserHandler(w http.ResponseWriter, r *http.Request) {
    // Extract ID from path
    parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
    idStr := parts[len(parts)-1]
    id, err := strconv.Atoi(idStr)
    
    if err != nil {
        http.Error(w, "Invalid user ID", http.StatusBadRequest)
        return
    }
    
    user := map[string]interface{}{
        "id": id,
        "name": "Alice",
        "email": "alice@example.com",
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(user)
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {
    var user map[string]interface{}
    if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    // In a real app, save to database
    user["id"] = 123
    
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(user)
}
```

## Using Templates

### Template Structure

Create `workers/blog/views/layout.html`:

```html
{{define "layout"}}
<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}}</title>
    <link rel="stylesheet" href="/blog/public/css/blog.css">
</head>
<body>
    <header>
        <h1>My Blog</h1>
        <nav>
            <a href="/blog/">Home</a>
            <a href="/blog/posts">Posts</a>
        </nav>
    </header>
    
    <main>
        {{template "content" .}}
    </main>
    
    <footer>
        <p>&copy; 2026 My Blog</p>
    </footer>
</body>
</html>
{{end}}
```

Create `workers/blog/views/home.html`:

```html
{{define "content"}}
<h2>Welcome to My Blog</h2>

<section class="recent-posts">
    <h3>Recent Posts</h3>
    {{range .Posts}}
    <article>
        <h4>{{.Title}}</h4>
        <p>{{.Content}}</p>
        <small>By {{.Author}}</small>
    </article>
    {{end}}
</section>
{{end}}
```

### Using Templates in Code

```go
import (
    "html/template"
    "net/http"
)

func renderTemplate(w http.ResponseWriter, name string, data interface{}) {
    tmpl := template.Must(template.ParseFiles(
        "views/layout.html",
        "views/" + name + ".html",
    ))
    
    if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
    }
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
    data := map[string]interface{}{
        "Title": "Home",
        "Posts": getRecentPosts(),
    }
    renderTemplate(w, "home", data)
}
```

## Serving Static Assets

### Public Directory

Files in `public/` are served directly:

```
workers/blog/public/
├── css/
│   └── blog.css         → /blog/public/css/blog.css
├── js/
│   └── app.js           → /blog/public/js/app.js
└── images/
    └── logo.png         → /blog/public/images/logo.png
```

### Referencing Assets

```html
<!-- In templates -->
<link rel="stylesheet" href="/blog/public/css/blog.css">
<script src="/blog/public/js/app.js"></script>
<img src="/blog/public/images/logo.png" alt="Logo">
```

## Worker Configuration

Create `workers/blog/config/worker.yaml`:

```yaml
# Path prefix for this worker (required)
path: "/blog"

# Worker runtime settings
runtime:
  go_max_procs: 2
  go_mem_limit: "512MiB"
  max_requests: 0

# Timeout settings
timeouts:
  request_timeout_seconds: 30
  idle_timeout_seconds: 120

# Logging
logging:
  log_file: "logs/worker_{name}_{date}.log"
```

See `config/worker.example.yaml` for all available options.

## Building and Running

### Automatic Building

TQServer automatically builds workers on startup and when files change:

```bash
# Start TQServer
./bin/tqserver

# Watch the logs
# Building worker: blog
# Successfully built worker: blog -> workers/blog/bin/blog
```

### Manual Building

Build a specific worker manually:

```bash
cd workers/blog
go build -o bin/blog ./src
```

### Testing the Worker

Test your worker:

```bash
# Home page
curl http://localhost:8080/blog/

# Posts
curl http://localhost:8080/blog/posts

# Health check
curl http://localhost:8080/blog/health
```

## Next Steps

- [Worker Lifecycle](lifecycle.md) - Understand the worker lifecycle
- [Worker Configuration](configuration.md) - Advanced configuration options
- [Templates](../basics/templates.md) - Learn the template engine
- [Routing](../basics/routing.md) - Advanced routing patterns
- [Testing Workers](testing.md) - Write tests for your workers
