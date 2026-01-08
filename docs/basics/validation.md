# Validation

Validating input is crucial for API security and integrity.

## Strict Decoding

When decoding JSON, you can assign default values or check for missing fields manually.

```go
type CreateUserRequest struct {
    Username string `json:"username"`
    Age      int    `json:"age"`
}

func createUser(w http.ResponseWriter, r *http.Request) {
    var req CreateUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    // Manual checks
    if req.Username == "" {
        http.Error(w, "Username required", http.StatusBadRequest)
        return
    }
    if req.Age < 18 {
        http.Error(w, "Must be 18+", http.StatusBadRequest)
        return
    }
}
```

## Validator Libraries

For complex validation, use libraries like `go-playground/validator`.

```go
import "github.com/go-playground/validator/v10"

type User struct {
    Username string `validate:"required,min=3,max=32"`
    Email    string `validate:"required,email"`
}

var validate = validator.New()

func createUser(w http.ResponseWriter, r *http.Request) {
    // ... decode ...
    
    if err := validate.Struct(req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
}
```
