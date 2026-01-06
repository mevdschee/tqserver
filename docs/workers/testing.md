# Testing Workers

- [Introduction](#introduction)
- [Testing Strategy](#testing-strategy)
- [Unit Testing](#unit-testing)
- [Integration Testing](#integration-testing)
- [End-to-End Testing](#end-to-end-testing)
- [Load Testing](#load-testing)
- [Test Configuration](#test-configuration)
- [CI/CD Integration](#cicd-integration)
- [Best Practices](#best-practices)

## Introduction

Comprehensive testing ensures workers are reliable, performant, and maintainable. TQServer workers can be tested at multiple levels from unit tests to full integration tests.

## Testing Strategy

### Test Pyramid

```
     /\
    /  \  E2E Tests (Few)
   /────\
  /      \  Integration Tests (Some)
 /────────\
/__________\  Unit Tests (Many)
```

**Test Distribution**:
- **70%** Unit Tests - Fast, isolated, comprehensive
- **20%** Integration Tests - Test component interactions
- **10%** E2E Tests - Test complete workflows

### Test Types

```go
// Unit Test - Test individual functions
func TestCalculateTotal(t *testing.T) {
    result := CalculateTotal(10, 5)
    if result != 15 {
        t.Errorf("Expected 15, got %d", result)
    }
}

// Integration Test - Test with real dependencies
func TestDatabaseConnection(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()
    
    err := db.Ping()
    if err != nil {
        t.Fatalf("Database connection failed: %v", err)
    }
}

// E2E Test - Test complete request flow
func TestAPIEndpoint(t *testing.T) {
    server := startTestServer(t)
    defer server.Close()
    
    resp, _ := http.Get(server.URL + "/api/users")
    if resp.StatusCode != 200 {
        t.Errorf("Expected 200, got %d", resp.StatusCode)
    }
}
```

## Unit Testing

### Basic Unit Tests

```go
// workers/api/src/calculator.go
package main

func Add(a, b int) int {
    return a + b
}

func Multiply(a, b int) int {
    return a * b
}
```

```go
// workers/api/src/calculator_test.go
package main

import "testing"

func TestAdd(t *testing.T) {
    tests := []struct {
        name     string
        a, b     int
        expected int
    }{
        {"positive numbers", 2, 3, 5},
        {"negative numbers", -2, -3, -5},
        {"zero", 0, 5, 5},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := Add(tt.a, tt.b)
            if result != tt.expected {
                t.Errorf("Add(%d, %d) = %d; want %d",
                    tt.a, tt.b, result, tt.expected)
            }
        })
    }
}

func TestMultiply(t *testing.T) {
    result := Multiply(3, 4)
    if result != 12 {
        t.Errorf("Multiply(3, 4) = %d; want 12", result)
    }
}
```

### Testing HTTP Handlers

```go
// workers/api/src/handlers.go
package main

import (
    "encoding/json"
    "net/http"
)

type Response struct {
    Message string `json:"message"`
    Status  string `json:"status"`
}

func HelloHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    response := Response{
        Message: "Hello, World!",
        Status:  "success",
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

```go
// workers/api/src/handlers_test.go
package main

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestHelloHandler(t *testing.T) {
    // Create request
    req := httptest.NewRequest(http.MethodGet, "/hello", nil)
    
    // Create response recorder
    rr := httptest.NewRecorder()
    
    // Call handler
    HelloHandler(rr, req)
    
    // Check status code
    if status := rr.Code; status != http.StatusOK {
        t.Errorf("Expected status 200, got %d", status)
    }
    
    // Check content type
    contentType := rr.Header().Get("Content-Type")
    if contentType != "application/json" {
        t.Errorf("Expected application/json, got %s", contentType)
    }
    
    // Check response body
    var response Response
    if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
        t.Fatalf("Failed to decode response: %v", err)
    }
    
    if response.Message != "Hello, World!" {
        t.Errorf("Expected 'Hello, World!', got '%s'", response.Message)
    }
    
    if response.Status != "success" {
        t.Errorf("Expected 'success', got '%s'", response.Status)
    }
}

func TestHelloHandler_InvalidMethod(t *testing.T) {
    req := httptest.NewRequest(http.MethodPost, "/hello", nil)
    rr := httptest.NewRecorder()
    
    HelloHandler(rr, req)
    
    if status := rr.Code; status != http.StatusMethodNotAllowed {
        t.Errorf("Expected 405, got %d", status)
    }
}
```

### Table-Driven Tests

```go
func TestUserValidation(t *testing.T) {
    tests := []struct {
        name    string
        user    User
        wantErr bool
        errMsg  string
    }{
        {
            name:    "valid user",
            user:    User{Name: "John", Email: "john@example.com", Age: 25},
            wantErr: false,
        },
        {
            name:    "empty name",
            user:    User{Name: "", Email: "john@example.com", Age: 25},
            wantErr: true,
            errMsg:  "name is required",
        },
        {
            name:    "invalid email",
            user:    User{Name: "John", Email: "invalid", Age: 25},
            wantErr: true,
            errMsg:  "invalid email format",
        },
        {
            name:    "negative age",
            user:    User{Name: "John", Email: "john@example.com", Age: -5},
            wantErr: true,
            errMsg:  "age must be positive",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateUser(tt.user)
            
            if tt.wantErr {
                if err == nil {
                    t.Error("Expected error, got nil")
                } else if err.Error() != tt.errMsg {
                    t.Errorf("Expected error '%s', got '%s'", tt.errMsg, err.Error())
                }
            } else {
                if err != nil {
                    t.Errorf("Unexpected error: %v", err)
                }
            }
        })
    }
}
```

### Mocking Dependencies

```go
// workers/api/src/database.go
package main

type Database interface {
    GetUser(id int) (*User, error)
    SaveUser(user *User) error
}

type UserService struct {
    db Database
}

func (s *UserService) GetUserByID(id int) (*User, error) {
    return s.db.GetUser(id)
}
```

```go
// workers/api/src/database_test.go
package main

import (
    "errors"
    "testing"
)

// Mock database
type MockDatabase struct {
    users map[int]*User
    err   error
}

func (m *MockDatabase) GetUser(id int) (*User, error) {
    if m.err != nil {
        return nil, m.err
    }
    user, ok := m.users[id]
    if !ok {
        return nil, errors.New("user not found")
    }
    return user, nil
}

func (m *MockDatabase) SaveUser(user *User) error {
    return m.err
}

func TestUserService_GetUserByID(t *testing.T) {
    // Setup mock
    mockDB := &MockDatabase{
        users: map[int]*User{
            1: {ID: 1, Name: "John"},
        },
    }
    
    service := &UserService{db: mockDB}
    
    // Test successful retrieval
    user, err := service.GetUserByID(1)
    if err != nil {
        t.Fatalf("Unexpected error: %v", err)
    }
    if user.Name != "John" {
        t.Errorf("Expected 'John', got '%s'", user.Name)
    }
    
    // Test not found
    _, err = service.GetUserByID(999)
    if err == nil {
        t.Error("Expected error, got nil")
    }
}

func TestUserService_GetUserByID_DatabaseError(t *testing.T) {
    mockDB := &MockDatabase{
        err: errors.New("database connection failed"),
    }
    
    service := &UserService{db: mockDB}
    
    _, err := service.GetUserByID(1)
    if err == nil {
        t.Error("Expected error, got nil")
    }
}
```

## Integration Testing

### Database Integration Tests

```go
// workers/api/src/integration_test.go
package main

import (
    "database/sql"
    "os"
    "testing"
    
    _ "github.com/lib/pq"
)

func setupTestDB(t *testing.T) *sql.DB {
    // Use test database
    dsn := os.Getenv("TEST_DATABASE_URL")
    if dsn == "" {
        t.Skip("TEST_DATABASE_URL not set")
    }
    
    db, err := sql.Open("postgres", dsn)
    if err != nil {
        t.Fatalf("Failed to connect to database: %v", err)
    }
    
    // Run migrations
    if err := runMigrations(db); err != nil {
        t.Fatalf("Failed to run migrations: %v", err)
    }
    
    return db
}

func cleanupTestDB(t *testing.T, db *sql.DB) {
    // Clean up test data
    _, err := db.Exec("TRUNCATE TABLE users CASCADE")
    if err != nil {
        t.Errorf("Failed to cleanup: %v", err)
    }
    db.Close()
}

func TestDatabaseIntegration(t *testing.T) {
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)
    
    // Insert test user
    result, err := db.Exec(
        "INSERT INTO users (name, email) VALUES ($1, $2)",
        "John", "john@example.com",
    )
    if err != nil {
        t.Fatalf("Failed to insert user: %v", err)
    }
    
    rowsAffected, _ := result.RowsAffected()
    if rowsAffected != 1 {
        t.Errorf("Expected 1 row affected, got %d", rowsAffected)
    }
    
    // Retrieve user
    var name, email string
    err = db.QueryRow("SELECT name, email FROM users WHERE name = $1", "John").
        Scan(&name, &email)
    if err != nil {
        t.Fatalf("Failed to query user: %v", err)
    }
    
    if name != "John" || email != "john@example.com" {
        t.Errorf("Expected John/john@example.com, got %s/%s", name, email)
    }
}
```

### Redis Integration Tests

```go
func TestRedisIntegration(t *testing.T) {
    redisURL := os.Getenv("TEST_REDIS_URL")
    if redisURL == "" {
        t.Skip("TEST_REDIS_URL not set")
    }
    
    client := redis.NewClient(&redis.Options{
        Addr: redisURL,
    })
    defer client.Close()
    
    ctx := context.Background()
    
    // Set value
    err := client.Set(ctx, "test_key", "test_value", 0).Err()
    if err != nil {
        t.Fatalf("Failed to set key: %v", err)
    }
    
    // Get value
    val, err := client.Get(ctx, "test_key").Result()
    if err != nil {
        t.Fatalf("Failed to get key: %v", err)
    }
    
    if val != "test_value" {
        t.Errorf("Expected 'test_value', got '%s'", val)
    }
    
    // Cleanup
    client.Del(ctx, "test_key")
}
```

## End-to-End Testing

### Full Request Flow Testing

```go
// workers/api/src/e2e_test.go
package main

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestE2E_CreateAndRetrieveUser(t *testing.T) {
    // Setup test server
    server := setupTestServer(t)
    defer server.Close()
    
    // Create user
    createReq := map[string]interface{}{
        "name":  "John Doe",
        "email": "john@example.com",
        "age":   30,
    }
    
    body, _ := json.Marshal(createReq)
    resp, err := http.Post(
        server.URL+"/api/users",
        "application/json",
        bytes.NewBuffer(body),
    )
    if err != nil {
        t.Fatalf("Failed to create user: %v", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusCreated {
        t.Errorf("Expected 201, got %d", resp.StatusCode)
    }
    
    // Parse response to get user ID
    var createResp struct {
        ID int `json:"id"`
    }
    json.NewDecoder(resp.Body).Decode(&createResp)
    
    // Retrieve user
    resp, err = http.Get(
        server.URL + "/api/users/" + fmt.Sprint(createResp.ID),
    )
    if err != nil {
        t.Fatalf("Failed to get user: %v", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        t.Errorf("Expected 200, got %d", resp.StatusCode)
    }
    
    // Verify user data
    var user User
    json.NewDecoder(resp.Body).Decode(&user)
    
    if user.Name != "John Doe" {
        t.Errorf("Expected 'John Doe', got '%s'", user.Name)
    }
    if user.Email != "john@example.com" {
        t.Errorf("Expected 'john@example.com', got '%s'", user.Email)
    }
}
```

### Testing with TQServer Running

```go
func TestE2E_WithTQServer(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping E2E test in short mode")
    }
    
    // TQServer should be running on localhost:8080
    baseURL := "http://localhost:8080"
    
    // Health check
    resp, err := http.Get(baseURL + "/health")
    if err != nil {
        t.Fatalf("TQServer not running: %v", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        t.Fatal("TQServer unhealthy")
    }
    
    // Test API endpoint
    resp, err = http.Get(baseURL + "/api/status")
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        t.Errorf("Expected 200, got %d", resp.StatusCode)
    }
}
```

## Load Testing

### Using Go's testing package

```go
func BenchmarkHandler(b *testing.B) {
    req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        rr := httptest.NewRecorder()
        HelloHandler(rr, req)
    }
}

// Run: go test -bench=. -benchmem
// Output:
// BenchmarkHandler-8    500000    2843 ns/op    1024 B/op    10 allocs/op
```

### Load Testing Script

```bash
#!/bin/bash
# scripts/load-test.sh

URL="http://localhost:8080/api/users"
CONCURRENT=100
REQUESTS=10000

echo "Load testing $URL"
echo "Concurrent: $CONCURRENT"
echo "Total requests: $REQUESTS"

ab -n $REQUESTS -c $CONCURRENT -k "$URL"

# Or use hey:
# hey -n $REQUESTS -c $CONCURRENT "$URL"
```

### Using k6 for Load Testing

```javascript
// tests/load/script.js
import http from 'k6/http';
import { check, sleep } from 'k6';

export let options = {
    stages: [
        { duration: '1m', target: 50 },   // Ramp up
        { duration: '3m', target: 50 },   // Stay at 50
        { duration: '1m', target: 100 },  // Ramp to 100
        { duration: '3m', target: 100 },  // Stay at 100
        { duration: '1m', target: 0 },    // Ramp down
    ],
};

export default function() {
    let res = http.get('http://localhost:8080/api/users');
    
    check(res, {
        'status is 200': (r) => r.status === 200,
        'response time < 500ms': (r) => r.timings.duration < 500,
    });
    
    sleep(1);
}
```

```bash
# Run k6 test
k6 run tests/load/script.js
```

## Test Configuration

### Test-Specific Config

```yaml
# workers/api/config.test.yaml

worker:
  environment:
    GO_ENV: "test"
    LOG_LEVEL: "debug"
    DATABASE_URL: "postgres://localhost/test_db"
    REDIS_URL: "redis://localhost:6379/1"
  
  resources:
    max_memory: "128M"  # Lower limits for tests
    max_cpu: 1.0
```

### Loading Test Config

```go
func setupTestConfig(t *testing.T) *Config {
    // Load test config
    config, err := LoadConfig("test")
    if err != nil {
        t.Fatalf("Failed to load test config: %v", err)
    }
    
    // Override with test-specific values
    config.DatabaseURL = os.Getenv("TEST_DATABASE_URL")
    if config.DatabaseURL == "" {
        config.DatabaseURL = "postgres://localhost/test_db"
    }
    
    return config
}
```

## CI/CD Integration

### GitHub Actions

```yaml
# .github/workflows/test.yml

name: Test Workers

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  test:
    runs-on: ubuntu-latest
    
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: test
          POSTGRES_DB: test_db
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        ports:
          - 5432:5432
      
      redis:
        image: redis:7
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        ports:
          - 6379:6379
    
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.24'
      
      - name: Install dependencies
        run: |
          cd workers/api
          go mod download
      
      - name: Run tests
        env:
          TEST_DATABASE_URL: postgres://postgres:test@localhost:5432/test_db?sslmode=disable
          TEST_REDIS_URL: localhost:6379
        run: |
          cd workers/api
          go test -v -coverprofile=coverage.out ./...
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./workers/api/coverage.out
```

### Makefile for Testing

```makefile
# Makefile

.PHONY: test test-unit test-integration test-e2e test-all test-coverage

# Run unit tests
test-unit:
	@echo "Running unit tests..."
	@cd workers/api && go test -v -short ./...

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	@cd workers/api && go test -v -run Integration ./...

# Run E2E tests
test-e2e:
	@echo "Running E2E tests..."
	@cd workers/api && go test -v -run E2E ./...

# Run all tests
test-all: test-unit test-integration test-e2e

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@cd workers/api && go test -coverprofile=coverage.out ./...
	@cd workers/api && go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: workers/api/coverage.html"

# Run tests in watch mode (requires fswatch)
test-watch:
	@fswatch -o workers/api/src | xargs -n1 -I{} make test-unit
```

## Best Practices

### Test Organization

```
workers/api/
├── src/
│   ├── main.go
│   ├── main_test.go           # Unit tests
│   ├── handlers.go
│   ├── handlers_test.go
│   ├── database.go
│   └── database_test.go
├── tests/
│   ├── integration/            # Integration tests
│   │   ├── database_test.go
│   │   └── redis_test.go
│   ├── e2e/                    # E2E tests
│   │   └── api_test.go
│   └── load/                   # Load tests
│       └── script.js
└── testdata/                   # Test fixtures
    ├── users.json
    └── responses/
```

### Test Naming Conventions

```go
// Function being tested: CalculateTotal
// Test function: TestCalculateTotal

// Sub-tests use descriptive names
func TestCalculateTotal(t *testing.T) {
    t.Run("positive_numbers", func(t *testing.T) { ... })
    t.Run("negative_numbers", func(t *testing.T) { ... })
    t.Run("zero_values", func(t *testing.T) { ... })
}

// Error cases
func TestCalculateTotal_InvalidInput(t *testing.T) { ... }
func TestCalculateTotal_Overflow(t *testing.T) { ... }
```

### Use Test Helpers

```go
// test_helpers.go
package main

import (
    "net/http/httptest"
    "testing"
)

func assertStatus(t *testing.T, got, want int) {
    t.Helper()
    if got != want {
        t.Errorf("Status: got %d, want %d", got, want)
    }
}

func assertBody(t *testing.T, got, want string) {
    t.Helper()
    if got != want {
        t.Errorf("Body: got %q, want %q", got, want)
    }
}

// Usage
func TestHandler(t *testing.T) {
    rr := httptest.NewRecorder()
    handler(rr, req)
    
    assertStatus(t, rr.Code, 200)
    assertBody(t, rr.Body.String(), "expected")
}
```

### Clean Up Resources

```go
func TestWithCleanup(t *testing.T) {
    // Setup
    db := setupDB(t)
    
    // Register cleanup
    t.Cleanup(func() {
        db.Close()
        cleanupTestData(db)
    })
    
    // Test code
    // Cleanup runs automatically even if test fails
}
```

## Next Steps

- [Health Checks](health-checks.md) - Implement health endpoints
- [Building Workers](building.md) - Build configuration
- [Deployment](../getting-started/deployment.md) - Deploy tested workers
- [Monitoring](../monitoring/metrics.md) - Monitor production workers
