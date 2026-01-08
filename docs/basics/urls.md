# URL Generation

Constructing URLs pointing to your application's resources.

## Dynamic Base URL

Avoid hardcoding `http://localhost:8080`. Read the base URL from config or environment.

```go
func absoluteURL(path string) string {
    baseURL := os.Getenv("APP_URL") 
    if baseURL == "" {
        baseURL = "http://localhost:8080"
    }
    return fmt.Sprintf("%s%s", baseURL, path)
}
```

## Route Helpers

If you use `gorilla/mux`, you can name your routes and reverse them.

```go
r := mux.NewRouter()
r.HandleFunc("/users/{id}", userHandler).Name("user_detail")

// In code
url, err := r.Get("user_detail").URL("id", "123")
// url.String() -> "/users/123"
```
