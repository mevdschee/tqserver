# Responses

Sends data back to the client.

## JSON Responses

Standard way to send JSON:

```go
func jsonHandler(w http.ResponseWriter, r *http.Request) {
    dto := map[string]string{"status": "ok"}
    
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK) // Optional, 200 is default
    json.NewEncoder(w).Encode(dto)
}
```

## HTML Responses

You can write HTML strings directly or serve files.

```go
func htmlHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html")
    fmt.Fprintf(w, "<h1>Hello World</h1>")
}
```

For templates, see the [Templates](templates.md) guide.

## Status Codes

Use `w.WriteHeader(code)` to set status codes.

> [!IMPORTANT]
> You must call `w.WriteHeader` *before* writing any data to the response body.

```go
func notFound(w http.ResponseWriter) {
    http.Error(w, "Not Found", http.StatusNotFound)
}
```

## Redirects

```go
func redirect(w http.ResponseWriter, r *http.Request) {
    http.Redirect(w, r, "/new-url", http.StatusFound)
}
```
