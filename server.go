package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

//go:embed ui/*
var uiFiles embed.FS

var (
	server    *http.Server
	mu        sync.Mutex
	boundHost = "127.0.0.1"
)

type Status struct {
	Status    string    `json:"status"`
	Port      int       `json:"port"`
	Uptime    string    `json:"uptime"`
	Version   string    `json:"version"`
	StartTime time.Time `json:"start_time"`
}

var serverStatus Status

func startServer(host string, port int) {
	serverStatus = Status{
		Status:    "running",
		Port:      port,
		Version:   Version,
		StartTime: time.Now(),
	}
	boundHost = host

	mux := http.NewServeMux()

	// Daemon lifecycle routes (cli-daemon-spec §2, §3).
	mux.HandleFunc("/_health", handleHealth)
	mux.HandleFunc("/_shutdown", handleShutdown)

	// The guide over HTTP (cli-guide-spec §3).
	mux.HandleFunc("/guide", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, guideJSON())
	})
	mux.HandleFunc("/llms.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, llmsTxt())
	})

	// App API (agent-first: JSON responses).
	mux.HandleFunc("/api/status", handleStatusAPI)
	mux.HandleFunc("/api/health", handleHealth)

	// Drive API. Every one of these needs a session; none of them lets the
	// browser hold a bkn token.
	bkn = newBkn(bknBase())
	sessions = newSessions(12 * time.Hour)
	mux.HandleFunc("/api/login", handleLogin) // rate limited inside, per IP and per account
	mux.HandleFunc("/api/logout", handleLogout)
	mux.HandleFunc("/api/_reset-limits", handleResetLimits) // loopback only
	mux.HandleFunc("/api/me", handleMe)
	mux.HandleFunc("/api/drive", throttled(handleDrive))
	mux.HandleFunc("/api/upload", throttled(handleUpload))
	mux.HandleFunc("/api/download", throttled(handleDownload))
	mux.HandleFunc("/api/preview", throttled(handlePreview))

	// Static UI files from the embedded filesystem.
	uiSub, _ := fs.Sub(uiFiles, "ui")
	mux.Handle("/", http.FileServer(http.FS(uiSub)))

	// net.JoinHostPort, not ":port": the bare form binds every interface, so a
	// server told to serve localhost would still be reachable from the whole
	// network (cli-daemon-spec §1).
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	server = &http.Server{Addr: addr, Handler: mux}

	// Startup lines are context — stderr, never stdout (cli-daemon-spec §1).
	fmt.Fprintf(os.Stderr, "bkn-drive serving on http://%s/\n", addr)
	fmt.Fprintf(os.Stderr, "  API: http://%s/api/status\n", addr)
	fmt.Fprintf(os.Stderr, "  bkn: %s\n", bkn.Base)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		die(ExitPrecondition, "port_unavailable",
			fmt.Sprintf("cannot bind %s: %v", addr, err),
			fmt.Sprintf("bkn-drive serve --port %d", port+1))
	}
}

// handleHealth is open and cheap: liveness only, no dependency checks (§2).
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"service": "bkn-drive",
		"pid":     os.Getpid(),
		"port":    serverStatus.Port,
	})
}

// handleShutdown answers before exiting, and is token-gated whenever the server
// is bound off-loopback — otherwise it is a remote kill switch (§3).
func handleShutdown(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(errBody(ExitUnknownCommand, "method_not_allowed", "POST /_shutdown"))
		return
	}

	if !shutdownAuthorized(r) {
		// 403, and the process MUST NOT stop.
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(errBody(ExitPrecondition, "forbidden",
			"X-Shutdown-Token required when bound off-loopback"))
		return
	}

	json.NewEncoder(w).Encode(map[string]any{"ok": true, "stopping": true})
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		os.Remove(pidFile)
		os.Exit(ExitOK)
	}()
}

func shutdownAuthorized(r *http.Request) bool {
	if isLoopback(boundHost) {
		return true
	}
	token := os.Getenv("SHUTDOWN_TOKEN")
	return token != "" && r.Header.Get("X-Shutdown-Token") == token
}

func isLoopback(host string) bool {
	switch host {
	case "127.0.0.1", "localhost", "::1":
		return true
	}
	return false
}

func errBody(code int, etype, message string) map[string]any {
	return map[string]any{
		"ok": false,
		"error": map[string]any{
			"code":        code,
			"type":        etype,
			"message":     message,
			"recoverable": false,
		},
	}
}

func handleStatusAPI(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	serverStatus.Uptime = time.Since(serverStatus.StartTime).String()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(serverStatus)
}
