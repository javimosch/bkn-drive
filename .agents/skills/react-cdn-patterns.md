# React 18 CDN Patterns Skill

## Overview

React 18 used via CDN with Babel standalone for JSX. All components in separate .jsx files.

## Setup

```html
<script crossorigin src="https://unpkg.com/react@18/umd/react.production.min.js"></script>
<script crossorigin src="https://unpkg.com/react-dom@18/umd/react-dom.production.min.js"></script>
<script src="https://unpkg.com/@babel/standalone/babel.min.js"></script>
<script src="https://unpkg.com/lucide@latest/dist/umd/lucide.js"></script>
<script src="https://cdn.tailwindcss.com"></script>
```

## Accessing React APIs

```javascript
const { useState, useEffect, useCallback, useContext, createContext, useMemo } = React;
```

## Component Pattern

```jsx
function MyComponent({ title, count, onUpdate }) {
    const [localState, setLocalState] = useState(0);

    useEffect(() => {
        lucide.createIcons();
    }, [count]);

    return (
        <div className="bg-white p-4 rounded-lg">
            <h2>{title}</h2>
            <p>{count + localState}</p>
            <button onClick={() => onUpdate(localState + 1)}>Update</button>
        </div>
    );
}

// Default props (optional)
MyComponent.defaultProps = {
    title: 'Default Title',
    count: 0
};
```

## State Management

### Local State

```jsx
const [count, setCount] = useState(0);
const increment = () => setCount(c => c + 1);
```

### Context for Global State

```jsx
// Create context
const AppContext = createContext(null);

// Custom hook
function useApp() {
    return useContext(AppContext);
}

// Provider in app.jsx
function App() {
    const [status, setStatus] = useState(null);
    
    return (
        <AppContext.Provider value={{ status, setStatus }}>
            <Routes>...</Routes>
        </AppContext.Provider>
    );
}

// Consumer in any component
function MyComponent() {
    const { status, setStatus } = useApp();
}
```

## Lifecycle (useEffect)

```jsx
// After mount
useEffect(() => {
    lucide.createIcons();
}, []);

// After dependency change
useEffect(() => {
    fetchData();
}, [dependency]);

// Cleanup on unmount
useEffect(() => {
    const interval = setInterval(fetchData, 5000);
    return () => clearInterval(interval);
}, []);
```

## Routing (Manual State)

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

## Lucide Icons

```jsx
{/* Use data-lucide attribute */}
<i data-lucide="settings" className="w-5 h-5"></i>

{/* Re-render after dynamic content */}
useEffect(() => {
    lucide.createIcons();
}, [dependencies]);
```

## Common Pitfalls

### 1. Script Load Order

Load in this exact order:
1. React
2. ReactDOM
3. Babel
4. Lucide
5. Tailwind
6. Components (Sidebar, StatusCard)
7. Views (Dashboard, Settings)
8. App (must be last)

### 2. No Import/Export

CDN React has no module system. All components are global:
```jsx
// Just define the function, no export
function MyComponent() { ... }

// Reference directly in other files
<MyComponent />
```

### 3. JSX Files Must Have type="text/babel"

```html
<!-- Correct -->
<script type="text/babel" src="/js/app.jsx"></script>

<!-- Wrong - won't transform JSX -->
<script src="/js/app.jsx"></script>
```

### 4. Use Functional Components

CDN React works best with functional components and hooks:
```jsx
// Good
function MyComponent() {
    const [state, setState] = useState(0);
    return <div>{state}</div>;
}

// Avoid class components with CDN
class MyComponent extends React.Component { ... }
```

### 5. Re-render Icons After State Changes

```jsx
function MyComponent() {
    const [data, setData] = useState(null);
    
    useEffect(() => {
        lucide.createIcons();
    }, [data]); // Re-render when data changes
    
    return <i data-lucide="icon" className="w-5 h-5"></i>;
}
```

## Adding a View

1. Create `ui/js/views/MyView.jsx`:
```jsx
function MyView() {
    const [data, setData] = useState(null);
    
    useEffect(() => {
        fetch('/api/my-data')
            .then(r => r.json())
            .then(setData);
    }, []);
    
    return <div>My View</div>;
}
```

2. Add route in `app.jsx`:
```jsx
<Route path="/my-view" element={<MyView />} />
```

3. Add nav item in `Sidebar.jsx`:
```jsx
{ id: 'my-view', label: 'My View', icon: 'star', path: '/my-view' }
```

4. Include in `index.html`:
```html
<script type="text/babel" src="/js/views/MyView.jsx"></script>
```
