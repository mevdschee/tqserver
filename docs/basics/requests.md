# Requests

Handling HTTP requests in TQServer workers involves using the standard Go `http.Request` object to parse headers, query parameters, bodies, and forms.

## JSON Body

To parse a JSON request body, define a struct matching the expected data and use `json.NewDecoder`.

```go
type CreateUserRequest struct {
    Username string `json:"username"`
    Email    string `json:"email"`
}

func createUser(w http.ResponseWriter, r *http.Request) {
    var req CreateUserRequest
    
    // Decode JSON body
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    // Use req.Username, req.Email...
}
```

## Query Parameters

Query parameters are accessible via `r.URL.Query()`.

```go
// GET /search?q=foo&limit=10
func search(w http.ResponseWriter, r *http.Request) {
    query := r.URL.Query().Get("q")
    limitStr := r.URL.Query().Get("limit")
    
    limit := 10 // default
    if limitStr != "" {
        limit, _ = strconv.Atoi(limitStr)
    }
}
```

## Form Data

To parse `application/x-www-form-urlencoded` or `multipart/form-data`:

```go
func submitForm(w http.ResponseWriter, r *http.Request) {
    // Parse form first (max memory 10MB)
    if err := r.ParseMultipartForm(10 << 20); err != nil {
        http.Error(w, "Form error", http.StatusBadRequest)
        return
    }
    
    username := r.FormValue("username")
    
    // File upload
    file, header, err := r.FormFile("avatar")
    if err == nil {
        defer file.Close()
        // Save file...
    }
}
```

## Headers

Access headers via `r.Header`.

```go
contentType := r.Header.Get("Content-Type")
userAgent := r.Header.Get("User-Agent")
```
