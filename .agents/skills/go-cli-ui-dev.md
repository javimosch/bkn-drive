# Go CLI UI Development Skill

## Overview

Development patterns for Go CLI with embedded React web UI.

## Key Patterns

### go:embed Setup

```go
import "embed"

//go:embed ui/*
var uiFiles embed.FS
```

### Serving Embedded Files

```go
uiSub, _ := fs.Sub(uiFiles, "ui")
fileServer := http.FileServer(http.FS(uiSub))
mux.Handle("/", fileServer)
```

### Adding API Endpoint

```go
// 1. Add handler
func handleMyAPI(w http.ResponseWriter, r *http.Request) {
    data := map[string]interface{}{"status": "ok"}
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(data)
}

// 2. Register in startServer()
mux.HandleFunc("/api/my-api", handleMyAPI)
```

## File Size Limits

- Go files: 500 LOC max
- React components: 300 LOC max
- CSS files: 300 LOC max

## Common Tasks

### Add New View

1. Create `ui/js/views/MyView.jsx`
2. Add route in `ui/js/app.jsx`: `<Route path="/my-view" element={<MyView />} />`
3. Add nav item in `ui/js/components/Sidebar.jsx`
4. Include script in `ui/index.html`: `<script type="text/babel" src="/js/views/MyView.jsx"></script>`

### Add New Component

1. Create `ui/js/components/MyComponent.jsx`
2. Include in `ui/index.html`
3. Use in views: `<MyComponent />`

### Add New API

1. Add handler function in `server.go`
2. Register route in `startServer()`
3. Test: `curl http://localhost:8080/api/my-api`

## Debugging

### Server won't start

- Check port: `lsof -i :8080`
- Check embed path: `ui/` must exist at project root

### UI not loading

- Verify `go:embed ui/*` path
- Check browser console for 404s
- Check script load order in index.html
- Try `go run .` (re-embeds files)

### Binary too large

- Check embedded files: `go list -m`
- Remove unused assets from `ui/`
- Build with: `go build -ldflags="-s -w"`

## Testing

```bash
# Run locally
go run .

# Test API
curl http://localhost:8080/api/status | jq

# Build binary
./build.sh

# Check binary
./boilerplate-cli-ui-go-v2-react version
```
