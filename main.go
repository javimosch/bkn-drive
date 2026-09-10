// Command bkn-drive is a CLI with an embedded web UI.
//
// The command surface follows the agent-first CLI specs
// (https://cli-specs.intrane.fr):
//
//	cli-output-spec  data on stdout, context on stderr, exit codes 80-119,
//	                 typed errors, help-json
//	cli-guide-spec   `guide`, embedded in the binary
//	cli-daemon-spec  `serve --host --port`, /_health, /_shutdown,
//	                 `daemon start|stop|status`
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

// Version is a var, not a const, so -ldflags "-X main.Version=..." can set it.
// As a const the linker flag is silently ignored and every build claims 1.0.0.
var Version = "1.0.0"

// Semantic exit codes (cli-output-spec §2).
const (
	ExitOK             = 0
	ExitMissingArg     = 80
	ExitUnknownCommand = 85
	ExitPrecondition   = 90
	ExitExternal       = 100
	ExitInternal       = 110
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(ExitMissingArg)
	}

	switch os.Args[1] {
	case "serve":
		host, port := serveFlags(os.Args[2:])
		startServer(host, port)
	case "daemon":
		handleDaemon()
	case "guide":
		handleGuide(os.Args[2:])
	case "help-json":
		fmt.Println(helpJSON())
	case "version":
		handleVersion(os.Args[2:])
	case "help", "--help", "-h":
		printHelp()

	// Back-compat aliases for the pre-spec command names.
	case "start":
		host, port := serveFlags(os.Args[2:])
		if hasFlag(os.Args[2:], "-daemon") || hasFlag(os.Args[2:], "--daemon") {
			daemonStart(host, port)
		} else {
			startServer(host, port)
		}
	case "stop":
		_, port := serveFlags(os.Args[2:])
		daemonStop(port)
	case "status":
		_, port := serveFlags(os.Args[2:])
		daemonStatus(port)

	default:
		die(ExitUnknownCommand, "unknown_command",
			fmt.Sprintf("unknown command %q", os.Args[1]),
			"bkn-drive help-json")
	}
}

// serveFlags resolves --host/--port. The host default MUST be loopback
// (cli-daemon-spec §1): serving the whole network is a deliberate act, never
// something that happens because nobody passed a flag.
func serveFlags(args []string) (string, int) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	defHost := envOr("HOST", "127.0.0.1")
	defPort := envOrInt("PORT", 8080)

	host := fs.String("host", defHost, "bind address")
	port := fs.Int("port", defPort, "port")
	// Accepted and ignored here; the alias path reads it directly.
	fs.Bool("daemon", false, "run in the background")

	if err := fs.Parse(args); err != nil {
		die(ExitMissingArg, "bad_flags", err.Error(),
			"bkn-drive help-json")
	}
	return *host, *port
}

func handleDaemon() {
	if len(os.Args) < 3 {
		die(ExitMissingArg, "missing_argument",
			"daemon needs a subcommand: start, stop or status",
			"bkn-drive daemon status")
	}
	sub := os.Args[2]
	host, port := serveFlags(os.Args[3:])

	switch sub {
	case "start":
		daemonStart(host, port)
	case "stop":
		daemonStop(port)
	case "status":
		daemonStatus(port)
	default:
		die(ExitUnknownCommand, "unknown_command",
			fmt.Sprintf("unknown daemon subcommand %q", sub),
			"bkn-drive daemon status")
	}
}

func handleGuide(args []string) {
	if hasFlag(args, "--human") {
		fmt.Println(guideMarkdown())
		return
	}
	fmt.Println(guideJSON())
}

func handleVersion(args []string) {
	if hasFlag(args, "--json") {
		out, _ := json.Marshal(map[string]string{
			"version": Version,
			"name":    "bkn-drive",
		})
		fmt.Println(string(out))
		return
	}
	fmt.Printf("bkn-drive v%s\n", Version)
}

// die emits a typed error on stdout and exits with the matching code. The exit
// status and .error.code are the same number by construction (§2, §3).
func die(code int, etype, message, suggestion string) {
	body := map[string]any{
		"ok": false,
		"error": map[string]any{
			"code":        code,
			"type":        etype,
			"message":     message,
			"recoverable": code >= 100 && code <= 109,
			"suggestions": []string{suggestion},
		},
	}
	out, _ := json.Marshal(body)
	fmt.Println(string(out))
	os.Exit(code)
}

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envOrInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return def
}

// printHelp writes to stderr: help is context, not the answer to a query, so
// stdout stays clean for data (cli-output-spec §1).
func printHelp() {
	e := os.Stderr
	fmt.Fprintln(e, "bkn-drive - Go CLI with an embedded React web UI")
	fmt.Fprintln(e)
	fmt.Fprintln(e, "Usage:")
	fmt.Fprintln(e, "  bkn-drive <command> [options]")
	fmt.Fprintln(e)
	fmt.Fprintln(e, "Commands:")
	fmt.Fprintln(e, "  serve [--host H] [--port N]   run the HTTP server in the foreground")
	fmt.Fprintln(e, "  daemon start [--port N]       start it in the background")
	fmt.Fprintln(e, "  daemon stop [--port N]        stop the background server")
	fmt.Fprintln(e, "  daemon status [--port N]      report background server status")
	fmt.Fprintln(e, "  guide [--human]               the embedded operator guide")
	fmt.Fprintln(e, "  help-json                     machine-readable command catalog")
	fmt.Fprintln(e, "  version [--json]              show version information")
	fmt.Fprintln(e, "  help                          show this help message")
	fmt.Fprintln(e)
	fmt.Fprintln(e, "Endpoints:")
	fmt.Fprintln(e, "  GET  /            Web UI")
	fmt.Fprintln(e, "  GET  /api/status  Server status (JSON)")
	fmt.Fprintln(e, "  GET  /_health     Liveness: {ok,service,pid}")
	fmt.Fprintln(e, "  POST /_shutdown   Stop the server (token-gated off-loopback)")
	fmt.Fprintln(e)
	fmt.Fprintln(e, "Exit codes: 0 ok, 80-89 input, 90-99 state, 100-109 external, 110-119 internal")
}
