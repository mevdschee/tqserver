# Views & Assets

Understanding where to place your HTML and static files is key to structuring a TQServer worker.

## Private vs Public

Worker directories are split into two concepts:

1.  **Private (`views/`)**: Server-side templates. These are NOT accessible via HTTP. They are read by your Go/PHP code and rendered.
2.  **Public (`public/`)**: Static assets (CSS, JS, Images). These ARE served directly to the browser by the TQServer Proxy.

## Directory Structure

```
workers/blog/
├── views/           # PRIVATE: HTML Templates
│   ├── home.html
│   └── partials/
│       └── footer.html
└── public/          # PUBLIC: Static Assets
    ├── css/
    │   └── style.css
    └── js/
        └── app.js
```

## Accessing Assets

Assets in `public/` are mapped to the worker's route.

-   File: `workers/blog/public/css/style.css`
-   URL: `/blog/public/css/style.css` 
    *(Note: The `public/` segment is currently part of the default path convention, though this depends on your proxy configuration)*.

> [!NOTE]
> Actually, strictly speaking, TQServer serves `workers/{name}/public/{path}` when the URL matches.
> If you request `/blog/css/style.css`, the proxy checks `workers/blog/public/css/style.css`.
> So usually the URL does **not** contain `public`. Assumes: `workers/blog/public/` maps to `/blog/`.

*Wait, let me verify the Proxy logic in `proxy.go`.*

```go
// From proxy.go
workerPublicPath := filepath.Join(p.projectRoot, p.config.Workers.Directory, worker.Name, "public", r.URL.Path)
```
If `r.URL.Path` is `/blog/style.css`, it looks in `workers/blog/public/blog/style.css`?
**NO.**
If worker route is `/blog`.
The `proxy.go` uses `r.URL.Path` directly?
`workerPublicPath := filepath.Join(..., "public", r.URL.Path)`
If checking for static file, it uses full path.
If I request `/blog/style.css`, it checks `workers/blog/public/blog/style.css`.
This implies assets inside `public` must be nested under a folder matching the route?
Or maybe I misread `proxy.go`.

Let's re-read `proxy.go` serving logic carefully before finalizing this doc.
