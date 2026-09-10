# AGENTS.md - Agent-First Go CLI with Embedded React UI

This document guides AI agents in understanding, extending, and maintaining this agent-first Go CLI boilerplate.

## Project Philosophy

This boilerplate implements **agent-first CLI design**:

- **JSON-by-default**: API endpoints return JSON
- **Structured errors**: Error objects with code, type, recoverable field
- **Output separation**: stdout for data, stderr for logs/progress
- **Single binary**: Frontend embedded via `go:embed`, no runtime dependencies
- **Agent-first HTTP**: JSON API at `/api/*`, UI at `/`

## Project Structure

```
bkn-drive/
├── main.go           # CLI entry point and command routing (max 500 LOC)
├── server.go         # HTTP server with go:embed (max 500 LOC)
├── daemon.go         # Process management (max 500 LOC)
├── ui/               # Frontend (embedded at compile time)
│   ├── index.html    # Entry point (React 18 CDN)
│   ├── css/
│   │   └── app.css   # Custom styles (max 300 LOC)
│   └── js/
│       ├── app.jsx   # React app with routing (max 300 LOC)
│       ├── components/
│       │   ├── Sidebar.jsx     # Navigation (max 200 LOC)
│       │   └── StatusCard.jsx  # Status display (max 200 LOC)
│       └── views/
│           ├── Dashboard.jsx   # Dashboard page (max 300 LOC)
│           └── Settings.jsx    # Settings page (max 300 LOC)
├── go.mod
├── build.sh
├── README.md
└── AGENTS.md         # This file
```

## Coding Rules

### File Size Limits

- **Max 500 LOC per Go file** - Split files that exceed this
- **Max 300 LOC per React component/view** - Keep components focused
- **Max 300 LOC per CSS file** - Use Tailwind for most styling

### Go File Organization

| File | Responsibility |
|------|----------------|
| `main.go` | CLI commands, flag parsing, help text |
| `server.go` | HTTP handlers, embedded filesystem, API responses |
| `daemon.go` | PID file management, process signals, daemon lifecycle |

### Frontend File Organization

| Directory | Responsibility |
|-----------|----------------|
| `js/app.jsx` | React app creation, routing, context providers |
| `js/components/` | Reusable UI components (sidebar, cards) |
| `js/views/` | Page-level components tied to routes |
| `css/` | Custom CSS that Tailwind can't handle |

### Naming Conventions

- **Go files**: `snake_case.go`
- **Go functions**: `PascalCase` exported, `camelCase` private
- **React components**: `PascalCase.jsx` (matches component name)
- **CSS classes**: Tailwind utilities + `kebab-case` for custom

## Key Pattern: `go:embed`

This boilerplate uses Go's embed directive to include frontend files in the binary:

```go
//go:embed ui/*
var uiFiles embed.FS
```

**How it works:**
1. All files under `ui/` are embedded at compile time
2. `http.FileServer` serves them as if they were on disk
3. Single binary output - no runtime file dependencies

**Development workflow:**
- Edit files in `ui/`
- Run `go run .` (files re-embed each run)
- Or serve from disk for faster iteration (see README)

## Adding New Views

### 1. Create View File

```jsx
// ui/js/views/MyView.jsx
function MyView() {
    const [data, setData] = React.useState(null);
    const [loading, setLoading] = React.useState(true);

    React.useEffect(() => {
        fetch('/api/my-endpoint')
            .then(r => r.json())
            .then(setData)
            .finally(() => setLoading(false));
    }, []);

    if (loading) return <div>Loading...</div>;

    return (
        <div>
            <h2 className="text-2xl font-bold text-gray-900">My View</h2>
            <pre>{JSON.stringify(data, null, 2)}</pre>
        </div>
    );
}
```

### 2. Add Route in app.jsx

```jsx
// ui/js/app.jsx

// Add to Routes component
<Route path="/my-view" element={<MyView />} />
```

### 3. Add Nav Item in Sidebar.jsx

```jsx
// ui/js/components/Sidebar.jsx

// Add to navItems array
{ id: 'my-view', label: 'My View', icon: 'star', path: '/my-view' }
```

### 4. Include in index.html

```html
<!-- ui/index.html -->
<script type="text/babel" src="/js/views/MyView.jsx"></script>
```

## Adding New API Endpoints

### 1. Add Handler in server.go

```go
func handleMyEndpoint(w http.ResponseWriter, r *http.Request) {
    data := map[string]interface{}{
        "result": "success",
        "timestamp": time.Now().UTC().Format(time.RFC3339),
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(data)
}
```

### 2. Register Route in startServer()

```go
mux.HandleFunc("/api/my-endpoint", handleMyEndpoint)
```

## Adding New Components

### 1. Create Component File

```jsx
// ui/js/components/MyComponent.jsx
function MyComponent({ title, value, onUpdate }) {
    return (
        <div className="bg-white rounded-xl border p-4">
            <h3 className="font-semibold">{title}</h3>
            <p className="text-2xl font-bold">{value}</p>
            <button onClick={onUpdate}>Refresh</button>
        </div>
    );
}
```

### 2. Include in index.html

```html
<script type="text/babel" src="/js/components/MyComponent.jsx"></script>
```

### 3. Use in Views

```jsx
// In any view
<MyComponent title="Status" value={status} onUpdate={refresh} />
```

## Agent-First Design Principles

### JSON API Responses

All `/api/*` endpoints must return JSON:

```go
// Always set content type
w.Header().Set("Content-Type", "application/json")

// Always return structured response
response := map[string]interface{}{
    "status": "success",
    "data": result,
    "timestamp": time.Now().UTC().Format(time.RFC3339),
}
json.NewEncoder(w).Encode(response)
```

### Error Responses

Use structured error format:

```go
func sendError(w http.ResponseWriter, message string, code int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    
    json.NewEncoder(w).Encode(map[string]interface{}{
        "error": map[string]interface{}{
            "code":    code,
            "message": message,
            "type":    "http_error",
        },
    })
}
```

### Output Separation

- **stdout**: JSON data, command results
- **stderr**: Logs, progress, errors
- **HTTP**: JSON responses only

```go
// Data to stdout
fmt.Println(string(jsonData))

// Logs to stderr
log.Printf("Processing item %d", i)

// Errors to stderr
fmt.Fprintf(os.Stderr, "Error: %v\n", err)
```

### Semantic Exit Codes

```
0     - Success
80-89 - User errors (invalid input, permission denied)
90-99 - Resource errors (not found, already exists)
100-109 - Integration errors (network, timeout)
110-119 - Software errors (internal, unexpected)
```

## Frontend Guidelines

### React 18 CDN Pattern

This boilerplate uses React 18 from CDN with Babel for JSX:

```html
<!-- Load React -->
<script crossorigin src="https://unpkg.com/react@18/umd/react.production.min.js"></script>
<script crossorigin src="https://unpkg.com/react-dom@18/umd/react-dom.production.min.js"></script>

<!-- Load Babel for JSX -->
<script src="https://unpkg.com/@babel/standalone/babel.min.js"></script>

<!-- Load components as text/babel -->
<script type="text/babel" src="/js/app.jsx"></script>
```

**Benefits:**
- No npm/webpack required
- Separate .jsx files with syntax highlighting
- Hot-reload by editing files and refreshing

### Component Pattern

```jsx
// Functional component with hooks
function MyComponent({ title, count }) {
    const [localState, setLocalState] = useState(0);
    
    useEffect(() => {
        // Side effects
        lucide.createIcons();
    }, [count]);
    
    return (
        <div>
            <h2>{title}</h2>
            <p>{count + localState}</p>
        </div>
    );
}

// With default props
MyComponent.defaultProps = {
    title: 'Default Title',
    count: 0
};
```

### State Management

**Local state:**
```jsx
const [count, setCount] = useState(0);
```

**Context for global state:**
```jsx
// Create context
const AppContext = createContext(null);

// Provider in app.jsx
<AppContext.Provider value={{ status, refresh }}>
    <Routes>...</Routes>
</AppContext.Provider>

// Consumer in any component
const { status, refresh } = useApp();
```

### Routing (Manual State)

```jsx
// In app.jsx - state-based routing
const [currentView, setCurrentView] = useState('dashboard');

// Navigation
function Sidebar() {
    const { handleNavigate } = useApp();
    return <button onClick={() => handleNavigate('settings')}>Go</button>;
}

// Render view conditionally
{currentView === 'dashboard' && <Dashboard />}
{currentView === 'settings' && <Settings />}
```

### Tailwind CSS

Use Tailwind utilities for all styling:

```jsx
{/* Good */}
<div className="bg-white rounded-xl border p-4 shadow-sm">

{/* Bad */}
<div className="my-card">
```

Custom CSS only for things Tailwind can't do (animations, transitions).

### Icon System (Lucide)

```jsx
{/* Use data-lucide attribute */}
<i data-lucide="settings" className="w-5 h-5"></i>

{/* Re-render after dynamic content */}
useEffect(() => {
    lucide.createIcons();
}, [dependencies]);
```

## Common Pitfalls

### Go

1. **embed paths are relative to go.mod** - `//go:embed ui/*` expects `ui/` at root
2. **Don't embed test files** - Use `//go:ignore` or separate directory
3. **File server needs fs.Sub** - Extract subdirectory before serving

```go
// Correct way to serve embedded subdirectory
uiSub, _ := fs.Sub(uiFiles, "ui")
fileServer := http.FileServer(http.FS(uiSub))
```

### React CDN

1. **Script load order matters** - React → ReactDOM → Router → Babel → Components → App
2. **Call lucide.createIcons()** after dynamic content changes
3. **Use `type="text/babel"`** for JSX files
4. **Components must be global** - No import/export with CDN

### go:embed

1. **Files are read at compile time** - Changes require rebuild
2. **Directory must exist** - Build fails if `ui/` missing
3. **Hidden files included** - Use `.gitignore` patterns carefully

## Development Workflow

### Adding a Feature

1. **Plan API endpoint** - Define JSON response structure
2. **Add Go handler** in `server.go`
3. **Create React component** in `js/components/` or `js/views/`
4. **Add route** in `app.jsx` if new view
5. **Add nav item** in `Sidebar.jsx` if new view
6. **Include script** in `index.html`
7. **Test both CLI and UI**

### Testing Checklist

- [ ] `go run .` starts server
- [ ] UI loads at `http://localhost:8080/`
- [ ] API returns JSON at `/api/status`
- [ ] New views render correctly
- [ ] Mobile responsive (test at 375px width)
- [ ] No console errors in browser
- [ ] Binary compiles: `./build.sh`
- [ ] Binary size reasonable (< 10MB)

## Extending the Boilerplate

### Adding Backend Dependencies

```bash
go get github.com/some/package
```

### Adding Frontend Libraries

Add CDN script to `ui/index.html`:

```html
<script src="https://unpkg.com/some-lib@latest"></script>
```

### Splitting Go Files

If a file exceeds 500 LOC:

1. Identify logical section
2. Create new file in same package
3. Move code
4. Verify: `go build .`

Example split for `server.go`:
```
server.go        - Main server setup
handlers.go      - HTTP handler functions
embed.go         - Embedded filesystem setup
```

## References

- [Go embed package](https://pkg.go.dev/embed)
- [React 18 CDN usage](https://react.dev/learn/add-react-to-a-website)
- [React Router DOM](https://reactrouter.com/)
- [Tailwind CSS CDN](https://tailwindcss.com/docs/installation/play-cdn)
- [Lucide Icons](https://lucide.dev/guide/installation)
- [Agent-friendly CLI design](https://clig.dev/)
