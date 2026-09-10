# go:embed Filesystem Patterns

## Overview

Go's `embed` package includes files in the binary at compile time.

## Basic Usage

```go
import "embed"

//go:embed ui/*
var uiFiles embed.FS
```

## Serving Files

### Method 1: FileServer (Recommended)

```go
uiSub, _ := fs.Sub(uiFiles, "ui")
fileServer := http.FileServer(http.FS(uiSub))
mux.Handle("/", fileServer)
```

### Method 2: Manual Read

```go
func handleIndex(w http.ResponseWriter, r *http.Request) {
    data, err := uiFiles.ReadFile("ui/index.html")
    if err != nil {
        http.Error(w, "Not found", 404)
        return
    }
    w.Header().Set("Content-Type", "text/html")
    w.Write(data)
}
```

## Path Rules

- Paths are relative to `go.mod` location
- `//go:embed ui/*` expects `ui/` directory at project root
- Use forward slashes, even on Windows
- No `..` allowed in paths

## Excluding Files

```go
// Only include specific types
//go:embed ui/*.html ui/css/*.css ui/js/*.js
var uiFiles embed.FS

// Or reorganize directory structure
```

## Listing Embedded Files

```go
import "io/fs"

func listFiles() {
    fs.WalkDir(uiFiles, ".", func(path string, d fs.DirEntry, err error) error {
        if !d.IsDir() {
            fmt.Println(path)
        }
        return nil
    })
}
```

## Reading Specific File

```go
data, err := uiFiles.ReadFile("ui/js/app.js")
if err != nil {
    log.Fatal(err)
}
```

## Common Patterns

### Content-Type Detection

```go
func contentType(path string) string {
    ext := filepath.Ext(path)
    switch ext {
    case ".html": return "text/html"
    case ".css": return "text/css"
    case ".js": return "application/javascript"
    case ".json": return "application/json"
    default: return "application/octet-stream"
    }
}
```

### SPA Fallback

```go
// Serve index.html for unknown paths (SPA routing)
func spaHandler(fileServer http.Handler) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Try to serve file
        path := "ui" + r.URL.Path
        if _, err := uiFiles.ReadFile(path); err != nil {
            // File not found, serve index.html
            r.URL.Path = "/"
        }
        fileServer.ServeHTTP(w, r)
    }
}
```

## Pitfalls

### 1. Directory Must Exist

Build fails if embedded directory is missing:
```
//go:embed ui/*
```
Error: `pattern ui/*: no matching files found`

### 2. Hidden Files Included

Dotfiles are embedded by default:
```
ui/.gitignore  // This gets embedded
```

### 3. Development Workflow

Files are read at compile time. For hot-reload:
- Use `go run .` (re-embeds each run)
- Or serve from disk during development:
  ```go
  // Temporarily replace embed with:
  fileServer := http.FileServer(http.Dir("ui"))
  ```

### 4. Binary Size

All embedded files increase binary size:
```bash
# Check what's embedded
go build -v .
ls -lh your-binary
```
