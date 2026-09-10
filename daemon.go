package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"
)

const (
	pidFile = "/tmp/bkn-drive.pid"
	logFile = "/tmp/bkn-drive.log"
)

// The tool has an embedded HTTP server, so /_health is the source of truth for
// whether the daemon is up — not the pid file, which can be stale
// (cli-daemon-spec §4, §6). Every subcommand is idempotent.

func healthURL(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d/_health", port)
}

func probeHealth(port int) bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(healthURL(port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// waitFor polls every 100ms for up to 5s, rather than sleeping a fixed amount
// and hoping (§4).
func waitFor(port int, want bool) bool {
	for i := 0; i < 50; i++ {
		if probeHealth(port) == want {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func emit(v any) {
	out, _ := json.Marshal(v)
	fmt.Println(string(out))
}

// daemonStart is idempotent: if the port is already healthy it reports the
// running instance and succeeds, instead of racing a second process onto it.
func daemonStart(host string, port int) {
	if probeHealth(port) {
		emit(map[string]any{"ok": true, "running": true, "already_running": true, "port": port})
		return
	}

	execPath, err := os.Executable()
	if err != nil {
		die(ExitInternal, "no_executable_path",
			fmt.Sprintf("cannot resolve own path: %v", err),
			"run the binary by an absolute path")
	}

	logHandle, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		die(ExitPrecondition, "log_unwritable",
			fmt.Sprintf("cannot open %s: %v", logFile, err),
			"check permissions on /tmp")
	}
	defer logHandle.Close()

	cmd := exec.Command(execPath, "serve",
		fmt.Sprintf("--host=%s", host), fmt.Sprintf("--port=%d", port))
	cmd.Stdout = logHandle
	cmd.Stderr = logHandle

	if err := cmd.Start(); err != nil {
		die(ExitInternal, "spawn_failed",
			fmt.Sprintf("cannot start the daemon: %v", err),
			fmt.Sprintf("bkn-drive serve --port %d", port))
	}

	pid := cmd.Process.Pid
	os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", pid)), 0644)

	if !waitFor(port, true) {
		cmd.Process.Kill()
		os.Remove(pidFile)
		die(ExitExternal, "daemon_unhealthy",
			fmt.Sprintf("started pid %d but /_health never answered on port %d (see %s)", pid, port, logFile),
			fmt.Sprintf("bkn-drive serve --port %d", port))
	}

	emit(map[string]any{
		"ok": true, "running": true, "already_running": false,
		"pid": pid, "port": port, "log": logFile,
	})
}

// daemonStop is a no-op success when nothing is running: an agent stopping an
// already-stopped daemon has got what it asked for (§4).
func daemonStop(port int) {
	if !probeHealth(port) {
		os.Remove(pidFile)
		emit(map[string]any{"ok": true, "running": false, "stopped": false, "port": port})
		return
	}

	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("http://127.0.0.1:%d/_shutdown", port), nil)
	if token := os.Getenv("SHUTDOWN_TOKEN"); token != "" {
		req.Header.Set("X-Shutdown-Token", token)
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		die(ExitExternal, "shutdown_failed",
			fmt.Sprintf("POST /_shutdown failed: %v", err),
			fmt.Sprintf("bkn-drive daemon status --port %d", port))
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		die(ExitExternal, "shutdown_refused",
			fmt.Sprintf("POST /_shutdown returned %d", resp.StatusCode),
			"set SHUTDOWN_TOKEN if the daemon is bound off-loopback")
	}

	waitFor(port, false)
	os.Remove(pidFile)
	emit(map[string]any{"ok": true, "running": false, "stopped": true, "port": port})
}

// daemonStatus only ever reads — it never carries the shutdown token (§4).
func daemonStatus(port int) {
	if !probeHealth(port) {
		emit(map[string]any{"ok": true, "running": false, "port": port})
		return
	}
	pid := 0
	if data, err := os.ReadFile(pidFile); err == nil {
		fmt.Sscanf(string(data), "%d", &pid)
	}
	emit(map[string]any{"ok": true, "running": true, "pid": pid, "port": port, "log": logFile})
}
