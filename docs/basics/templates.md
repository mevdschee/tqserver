# Templates

TQServer workers typically use Go's standard `html/template` package to render dynamic HTML.

## Basic Usage

1.  **Parse**: Load template files from disk.
2.  **Execute**: Render the template with data to an `io.Writer` (usually `http.ResponseWriter`).

```go
func handler(w http.ResponseWriter, r *http.Request) {
    // Parse
    tmpl, err := template.ParseFiles("views/page.html")
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    // Execute
    data := map[string]string{"Title": "Home"}
    tmpl.Execute(w, data)
}
```

## Template Syntax

Go templates use `{{}}` logic.

### Variables
```html
<h1>{{.Title}}</h1>
<p>Hello, {{.User.Name}}</p>
```

### Directives

**Conditionals**
```html
{{if .IsAdmin}}
    <button>Delete</button>
{{else}}
    <span>Read Only</span>
{{end}}
```

**Loops**
```html
<ul>
    {{range .Items}}
        <li>{{.}}</li>
    {{end}}
</ul>
```

## Layouts

To share headers/footers, define blocks.

`views/layout.html`:
```html
{{define "layout"}}
<html>
<body>
    {{template "content" .}}
</body>
</html>
{{end}}
```

`views/home.html`:
```html
{{define "content"}}
    <h1>Home Page</h1>
{{end}}
```

**Rendering:**
```go
tmpl := template.Must(template.ParseFiles("views/layout.html", "views/home.html"))
tmpl.ExecuteTemplate(w, "layout", data)
```

## Caching

For performance, parse templates once at startup (global variable or struct field) instead of on every request.

```go
var templates = template.Must(template.ParseGlob("views/*.html"))

func handler(w http.ResponseWriter, r *http.Request) {
    templates.ExecuteTemplate(w, "layout", nil)
}
```
