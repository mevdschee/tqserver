# Error Handling

Pattern for consistent error responses.

## Centralized Error Helper

Define a helper to ensure all errors share the same format.

```go
type ErrorResponse struct {
    Error string `json:"error"`
    Code  int    `json:"code,omitempty"`
}

func RespondError(w http.ResponseWriter, statusCode int, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}
```

## Usage

```go
func handler(w http.ResponseWriter, r *http.Request) {
    if err := doSomething(); err != nil {
        log.Printf("Internal error: %v", err) // Log details internally
        RespondError(w, http.StatusInternalServerError, "Internal Server Error")
        return
    }
}
```

## Panic Recovery

Standard Go HTTP servers recover from panics for you, but you can add middleware to log the stack trace gracefully and return a 500 JSON response instead of an empty connection reset.
