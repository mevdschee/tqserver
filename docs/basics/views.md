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

Assets in `public/` are served using the request's full URL path.

**Important**: Because the proxy includes the worker's route prefix in the file lookup, you must nest your assets under a directory matching your route name.

### Example: Worker mounted at `/blog`

Structure:
```
workers/blog/public/
└── blog/              <-- Nest under route name
    └── css/
        └── style.css
```

-   **URL**: `/blog/css/style.css`
-   **Lookup**: `workers/blog/public/blog/css/style.css`

### Example: Worker mounted at `/` (index)

Structure:
```
workers/index/public/
└── css/
    └── style.css
```

-   **URL**: `/css/style.css`
-   **Lookup**: `workers/index/public/css/style.css`


