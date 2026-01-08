# Controllers

In Go web applications, "Controllers" are typically grouping of HTTP handlers that share dependencies (like database connections or services).

## Struct-Based Handlers

The recommended pattern is to define a struct for your controller and attach methods as handlers.

```go
type UserController struct {
    DB *sql.DB
    // other dependencies
}

func NewUserController(db *sql.DB) *UserController {
    return &UserController{DB: db}
}

func (c *UserController) List(w http.ResponseWriter, r *http.Request) {
    // Access c.DB here
    users := c.getUsersFromDB()
    json.NewEncoder(w).Encode(users)
}

func (c *UserController) Show(w http.ResponseWriter, r *http.Request) {
    // ...
}
```

## Registering Routes

You can then register these methods as routes in your `main()` function:

```go
func main() {
    db := connectDB()
    userController := NewUserController(db)

    http.HandleFunc("/users", userController.List)
    http.HandleFunc("/users/detail", userController.Show)
    
    // ...
}
```

## Organizing Code

For larger workers, organize controllers into their own packages.

```
workers/api/src/
├── main.go
├── controllers/
│   ├── user_controller.go
│   └── post_controller.go
├── models/
│   └── user.go
└── services/
```

This keeps your `main.go` clean and focused on wiring up dependencies and routes.
