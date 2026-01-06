# Starter Kits

- [Introduction](#introduction)
- [Available Starter Kits](#available-starter-kits)
- [Creating a New Project](#creating-a-new-project)
- [Project Templates](#project-templates)
- [Customizing Your Project](#customizing-your-project)

## Introduction

Starter kits provide pre-configured project templates to help you get started quickly with TQServer. Each kit includes a complete project structure, example workers, and best practices.

## Available Starter Kits

### Basic Web Application

A simple web application with HTML templates and static assets.

**Use Case**: Personal website, blog, documentation site

**Features**:
- Homepage worker
- Template rendering
- Static assets (CSS, JS)
- Basic routing
- Example views

**Quick Start**:
```bash
# Clone the starter kit
git clone https://github.com/tqserver/starter-basic.git myproject
cd myproject

# Install TQServer
go mod download

# Build and run
./scripts/build-dev.sh
./bin/tqserver
```

**Structure**:
```
myproject/
├── workers/
│   └── web/
│       ├── src/
│       │   ├── main.go
│       │   └── handlers.go
│       ├── views/
│       │   ├── layout.html
│       │   ├── home.html
│       │   └── about.html
│       ├── config/
│       │   └── worker.yaml
│       └── public/
│           ├── css/style.css
│           └── js/app.js
├── config/
│   └── server.yaml
└── README.md
```

### REST API Starter

A RESTful API with JSON responses, middleware, and examples.

**Use Case**: Backend API, microservice, mobile app backend

**Features**:
- RESTful routes
- JSON responses
- Authentication middleware
- CORS support
- Request validation
- Error handling
- Health checks

**Quick Start**:
```bash
git clone https://github.com/tqserver/starter-api.git myapi
cd myapi
go mod download
./scripts/build-dev.sh
./bin/tqserver

# Test the API
curl http://localhost:8080/api/health
curl http://localhost:8080/api/users
```

**Structure**:
```
myapi/
├── workers/
│   └── api/
│       └── src/
│           ├── main.go
│           ├── routes.go
│           ├── handlers/
│           │   ├── users.go
│           │   ├── posts.go
│           │   └── health.go
│           ├── middleware/
│           │   ├── auth.go
│           │   ├── cors.go
│           │   └── logger.go
│           ├── models/
│           │   ├── user.go
│           │   └── post.go
│           └── utils/
│               └── response.go
└── config/
    └── server.yaml
```

### Full-Stack Application

Complete full-stack app with web frontend and API backend.

**Use Case**: Web application, SaaS product, admin dashboard

**Features**:
- Web frontend worker
- API backend worker
- Authentication
- Database integration
- Session management
- Asset pipeline
- Email support

**Quick Start**:
```bash
git clone https://github.com/tqserver/starter-fullstack.git myapp
cd myapp

# Setup database
createdb myapp_dev
go run cmd/migrate/main.go up

# Start server
go mod download
./scripts/build-dev.sh
./bin/tqserver
```

**Structure**:
```
myapp/
├── workers/
│   ├── web/              # Frontend
│   │   ├── src/
│   │   ├── views/
│   │   └── public/assets/
│   └── api/              # Backend API
│       └── src/
├── pkg/
│   ├── database/
│   ├── auth/
│   └── email/
├── migrations/
└── config/
```

### Microservices Template

Multiple coordinated services for distributed systems.

**Use Case**: Large applications, microservices architecture

**Features**:
- Multiple service workers
- Service discovery
- Inter-service communication
- Shared libraries
- Distributed tracing
- Health monitoring

**Quick Start**:
```bash
git clone https://github.com/tqserver/starter-microservices.git myservices
cd myservices
docker-compose up -d  # Start dependencies
./scripts/build-all.sh
./bin/tqserver
```

**Structure**:
```
myservices/
├── workers/
│   ├── gateway/       # API Gateway
│   ├── auth/          # Auth service
│   ├── users/         # User service
│   ├── orders/        # Order service
│   └── notifications/ # Notification service
├── pkg/
│   ├── common/        # Shared code
│   └── proto/         # Service definitions
└── docker-compose.yml
```

## Creating a New Project

### Using the CLI (Planned)

```bash
# Install TQServer CLI
go install github.com/mevdschee/tqserver/cmd/tqcli@latest

# Create new project
tqcli new myproject --template=basic

# Or interactive mode
tqcli new
? Project name: myproject
? Template: basic-web
? Enable database: yes
? Database type: postgresql
```

### Manual Setup

```bash
# Create project directory
mkdir myproject
cd myproject

# Initialize Go module
go mod init github.com/username/myproject

# Add TQServer dependency
go get github.com/mevdschee/tqserver@latest

# Create directory structure
mkdir -p workers/index/src
mkdir -p workers/index/views
mkdir -p workers/index/config
mkdir -p workers/index/data
mkdir -p workers/index/public/css
mkdir -p config
mkdir -p bin

# Create configuration
cp $GOPATH/pkg/mod/github.com/mevdschee/tqserver@*/config/server.example.yaml config/server.yaml
```

## Project Templates

### Minimal Template

The bare minimum to get started:

```go
// workers/index/src/main.go
package main

import (
    "fmt"
    "log"
    "net/http"
    "os"
)

func main() {
    port := os.Getenv("PORT")
    if port == "" {
        port = "9000"
    }
    
    http.HandleFunc("/", homeHandler)
    http.HandleFunc("/health", healthHandler)
    
    log.Printf("Worker starting on port %s", port)
    log.Fatal(http.ListenAndServe(":"+port, nil))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "<h1>Hello, TQServer!</h1>")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    fmt.Fprintf(w, "OK")
}
```

```yaml
# config/server.yaml
server:
  port: 3000

workers:
  directory: "workers"
  port_range_start: 10000
  port_range_end: 19999
```

### API Template

Basic REST API structure:

```go
// workers/api/src/main.go
package main

import (
    "encoding/json"
    "log"
    "net/http"
    "os"
)

type Response struct {
    Message string                 `json:"message"`
    Data    interface{}            `json:"data,omitempty"`
    Error   string                 `json:"error,omitempty"`
}

func main() {
    port := os.Getenv("PORT")
    
    http.HandleFunc("/", indexHandler)
    http.HandleFunc("/users", usersHandler)
    http.HandleFunc("/health", healthHandler)
    
    log.Fatal(http.ListenAndServe(":"+port, nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
    respondJSON(w, http.StatusOK, Response{
        Message: "API is running",
    })
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        users := []map[string]interface{}{
            {"id": 1, "name": "Alice"},
            {"id": 2, "name": "Bob"},
        }
        respondJSON(w, http.StatusOK, Response{
            Data: users,
        })
    case http.MethodPost:
        var user map[string]interface{}
        if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
            respondError(w, http.StatusBadRequest, "Invalid request body")
            return
        }
        user["id"] = 3
        respondJSON(w, http.StatusCreated, Response{
            Data: user,
        })
    default:
        respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
    }
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
    respondJSON(w, http.StatusOK, Response{
        Message: "healthy",
    })
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, message string) {
    respondJSON(w, status, Response{
        Error: message,
    })
}
```

## Customizing Your Project

### Adding a New Worker

```bash
# Create worker structure
mkdir -p workers/admin/{src,views,config,data,public/css}

# Create main.go
cat > workers/admin/src/main.go << 'EOF'
package main

import (
    "log"
    "net/http"
    "os"
)

func main() {
    port := os.Getenv("PORT")
    http.HandleFunc("/", adminHandler)
    http.HandleFunc("/health", healthHandler)
    log.Fatal(http.ListenAndServe(":"+port, nil))
}

func adminHandler(w http.ResponseWriter, r *http.Request) {
    // Admin logic
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
}
EOF

# Build
go build -o workers/admin/bin/admin workers/admin/src/*.go
```

### Adding Database Support

```bash
# Add database package
go get github.com/lib/pq

# Create database package
mkdir -p pkg/database
```

```go
// pkg/database/db.go
package database

import (
    "database/sql"
    "fmt"
    
    _ "github.com/lib/pq"
)

func Connect(host, port, user, password, dbname string) (*sql.DB, error) {
    connStr := fmt.Sprintf(
        "host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
        host, port, user, password, dbname,
    )
    
    db, err := sql.Open("postgres", connStr)
    if err != nil {
        return nil, err
    }
    
    if err := db.Ping(); err != nil {
        return nil, err
    }
    
    return db, nil
}
```

### Adding Authentication

```go
// pkg/auth/jwt.go
package auth

import (
    "errors"
    "time"
    
    "github.com/golang-jwt/jwt/v5"
)

type Claims struct {
    UserID int `json:"user_id"`
    jwt.RegisteredClaims
}

func GenerateToken(userID int, secret string) (string, error) {
    claims := Claims{
        UserID: userID,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(secret))
}

func ValidateToken(tokenString, secret string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        return []byte(secret), nil
    })
    
    if err != nil {
        return nil, err
    }
    
    if claims, ok := token.Claims.(*Claims); ok && token.Valid {
        return claims, nil
    }
    
    return nil, errors.New("invalid token")
}
```

### Environment-Specific Configuration

```yaml
# config/server.dev.yaml
server:
  port: 3000

workers:
  directory: "workers"

file_watcher:
  debounce_ms: 50
```

```yaml
# config/server.prod.yaml
server:
  port: 80
  log_file: "/var/log/tqserver/server_{date}.log"

workers:
  directory: "workers"
```

```bash
# Run with specific config
./bin/tqserver -config=config/server.prod.yaml
```

## Best Practices

1. **Keep workers focused** - Each worker should have a single responsibility
2. **Use shared packages** - Common code goes in `pkg/`
3. **Environment variables** - Use env vars for secrets and environment-specific config
4. **Health checks** - Always implement health check endpoints
5. **Error handling** - Handle errors gracefully and log appropriately
6. **Testing** - Write tests for your handlers and logic
7. **Documentation** - Document your API endpoints and configuration

## Next Steps

- [Configuration](configuration.md) - Configure your project
- [Creating Workers](../workers/creating.md) - Build custom workers
- [Deployment](deployment.md) - Deploy to production
- [Best Practices](../appendix/best-practices.md) - Learn recommended patterns
