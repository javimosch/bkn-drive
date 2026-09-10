package main

import "encoding/json"

// The embedded guide and command catalog (cli-guide-spec, cli-output-spec §4).
//
// Compiled into the binary: an agent that lands on a machine with this binary
// and no network can still learn the tool. Never fetch this at runtime.

func guideJSON() string {
	g := map[string]any{
		"bkn-drive": "A Go CLI with an embedded React web UI, compiled to a single binary.",
		"version":                  Version,
		"one_liner": "Starts an HTTP server that serves a React 18 dashboard at / and a JSON API at /api/*, " +
			"from one static binary with the UI embedded via go:embed — no assets to deploy alongside it.",
		"model": map[string]string{
			"binary":   "one static executable; the UI ships inside it through go:embed.",
			"server":   "net/http with an explicit mux; the bind address is host:port, never :port.",
			"daemon":   "re-execs itself as `serve`, detached, with /_health as the source of truth for liveness.",
			"contract": "agent-first: data on stdout, context on stderr, semantic exit codes, typed errors, an embedded guide.",
		},
		"loop": []string{
			"./build.sh — compile the binary with the UI embedded",
			"./bkn-drive serve — foreground on 127.0.0.1:8080",
			"open http://127.0.0.1:8080/ for the UI, or curl /api/status for JSON",
			"./bkn-drive daemon start — background it instead",
			"./bkn-drive daemon stop — stop it",
		},
		"concepts": map[string]string{
			"embedded UI":      "//go:embed ui/* compiles the ui/ directory into the binary. Edit the files, rebuild.",
			"loopback default": "serve binds 127.0.0.1 unless --host says otherwise. Binding the whole network is deliberate.",
			"shutdown token":   "off-loopback, POST /_shutdown requires X-Shutdown-Token matching $SHUTDOWN_TOKEN, or it answers 403 and keeps running.",
			"exit codes":       "0 ok, 80-89 input, 90-99 state, 100-109 external, 110-119 internal. The code equals .error.code in the body.",
		},
		"commands": map[string][]string{
			"server": {
				"bkn-drive serve [--host H] [--port N]",
				"bkn-drive daemon start [--port N]",
				"bkn-drive daemon stop [--port N]",
				"bkn-drive daemon status [--port N]",
			},
			"introspection": {
				"bkn-drive guide [--human]",
				"bkn-drive help-json",
				"bkn-drive version [--json]",
			},
		},
		"examples": []map[string]any{
			{"goal": "serve the UI on a custom port", "do": []string{"./bkn-drive serve --port 3000"}},
			{"goal": "background it and confirm it is up", "do": []string{
				"./bkn-drive daemon start --port 3000",
				"./bkn-drive daemon status --port 3000",
			}},
			{"goal": "expose it on the LAN with a kill switch that needs a token", "do": []string{
				"SHUTDOWN_TOKEN=s3cret ./bkn-drive serve --host 0.0.0.0 --port 8080",
			}},
		},
		"gotchas": []string{
			"The UI is compiled in: editing ui/ does nothing until you rebuild.",
			"serve binds 127.0.0.1 by default. If you expected it on the LAN, pass --host 0.0.0.0 — and then set SHUTDOWN_TOKEN, or /_shutdown answers 403 to everyone.",
			"daemon start is idempotent: called twice it reports the running instance instead of racing a second process onto the port.",
			"daemon stop against a stopped daemon exits 0 — a no-op success, not an error.",
			"Startup lines go to stderr. An agent parsing stdout sees only data.",
		},
		"see_also": []string{"https://cli-specs.intrane.fr"},
	}
	out, _ := json.Marshal(g)
	return string(out)
}

func guideMarkdown() string {
	return `# bkn-drive

A Go CLI with an embedded React web UI, compiled to a single static binary.

## Model

- One executable; the UI ships inside it through ` + "`go:embed`" + `.
- net/http with an explicit mux, bound to host:port (never bare :port).
- The daemon re-execs itself as ` + "`serve`" + `, detached; /_health is liveness.
- Agent-first: data on stdout, context on stderr, semantic exit codes.

## Loop

1. ` + "`./build.sh`" + `
2. ` + "`./bkn-drive serve`" + `
3. Open http://127.0.0.1:8080/ or curl /api/status.
4. ` + "`./bkn-drive daemon start`" + ` to background it.
5. ` + "`./bkn-drive daemon stop`" + ` to stop it.

## Commands

- ` + "`serve [--host H] [--port N]`" + `
- ` + "`daemon start|stop|status [--port N]`" + `
- ` + "`guide [--human]`" + `, ` + "`help-json`" + `, ` + "`version [--json]`" + `

## Gotchas

- The UI is compiled in: rebuild after editing ui/.
- ` + "`serve`" + ` binds 127.0.0.1 by default; ` + "`--host 0.0.0.0`" + ` is deliberate.
- Off-loopback, ` + "`POST /_shutdown`" + ` needs ` + "`X-Shutdown-Token`" + ` = ` + "`$SHUTDOWN_TOKEN`" + `.
- ` + "`daemon start`" + ` twice is idempotent; ` + "`daemon stop`" + ` when stopped exits 0.
`
}

func llmsTxt() string {
	return `# bkn-drive

A Go CLI with an embedded React web UI. One static binary.

## Drive it

    bkn-drive serve [--host H] [--port N]
    bkn-drive daemon start|stop|status [--port N]

JSON on stdout, context on stderr, exit 0/80-119.

## Learn it

    bkn-drive guide      # embedded, JSON
    bkn-drive help-json  # command catalog

HTTP: GET /  GET /api/status  GET /_health  POST /_shutdown  GET /guide
`
}

func helpJSON() string {
	c := map[string]any{
		"version":      "1.0",
		"tool":         "bkn-drive",
		"tool_version": Version,
		"commands": []map[string]any{
			{"name": "serve", "summary": "run the HTTP server in the foreground", "flags": []map[string]any{
				{"name": "--host", "summary": "bind address", "default": "127.0.0.1", "env": "HOST"},
				{"name": "--port", "summary": "port", "default": "8080", "env": "PORT"},
			}},
			{"name": "daemon start", "summary": "start the server in the background (idempotent)"},
			{"name": "daemon stop", "summary": "stop the background server (no-op success if stopped)"},
			{"name": "daemon status", "summary": "report background server status"},
			{"name": "guide", "summary": "the embedded operator guide", "flags": []map[string]any{
				{"name": "--human", "summary": "markdown instead of JSON"},
			}},
			{"name": "help-json", "summary": "this machine-readable command catalog"},
			{"name": "version", "summary": "print the version", "flags": []map[string]any{
				{"name": "--json", "summary": "JSON output"},
			}},
		},
		"endpoints": []map[string]string{
			{"method": "GET", "path": "/", "summary": "the embedded web UI"},
			{"method": "GET", "path": "/api/status", "summary": "app status JSON"},
			{"method": "GET", "path": "/_health", "summary": "liveness: {ok,service,pid}"},
			{"method": "POST", "path": "/_shutdown", "summary": "stop the server; token-gated off-loopback"},
			{"method": "GET", "path": "/guide", "summary": "the guide over HTTP"},
			{"method": "GET", "path": "/llms.txt", "summary": "the short agent-facing README"},
		},
		"exit_codes": map[string]string{
			"0":   "success",
			"80":  "missing argument or bad flags",
			"85":  "unknown command",
			"90":  "precondition failed (port unavailable, forbidden)",
			"100": "external failure (the daemon did not answer)",
			"110": "internal error",
		},
		"env": []map[string]string{
			{"name": "PORT", "summary": "default port"},
			{"name": "HOST", "summary": "default bind address"},
			{"name": "SHUTDOWN_TOKEN", "summary": "required by POST /_shutdown when bound off-loopback"},
		},
		"see_also": []string{"bkn-drive guide", "https://cli-specs.intrane.fr"},
	}
	out, _ := json.Marshal(c)
	return string(out)
}
