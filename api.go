package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// MaxUpload matches the drive-upload hook's max_bytes. Rejecting here as well
// means a too-large file fails immediately and locally, instead of after the
// browser has spent minutes pushing it at bkn.
const MaxUpload = 25 << 20

var (
	bkn      *bknClient
	sessions *sessionStore

	// Two limiters on sign-in, because they stop different attacks. Per-IP
	// stops one host guessing many passwords; per-account stops many hosts
	// guessing one account's password, which per-IP alone would wave through.
	loginByIP      = newLimiter(8, 5*time.Minute)
	loginByAccount = newLimiter(12, 15*time.Minute)

	// The rest of the API is authenticated, so this is a ceiling on damage
	// rather than a gate: generous enough that real use never notices.
	apiByIP = newLimiter(240, time.Minute)
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeAPIErr(w http.ResponseWriter, err error) {
	if errors.Is(err, errNoSession) || errors.Is(err, errUnauthorized) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"ok": false, "error": map[string]any{
				"type": "not_signed_in", "message": "sign in to continue", "recoverable": true,
			}})
		return
	}
	var ae *apiError
	if errors.As(err, &ae) {
		status := ae.Status
		if status == 0 || status < 400 {
			status = http.StatusBadGateway
		}
		writeJSON(w, status, map[string]any{
			"ok": false, "error": map[string]any{
				"type": ae.Type, "message": ae.Message, "recoverable": status < 500,
			}})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]any{
		"ok": false, "error": map[string]any{
			"type": "internal_error", "message": err.Error(), "recoverable": false,
		}})
}

// withBkn runs fn with a valid access token, refreshing once if bkn says the
// token expired. Access tokens live 15 minutes, so without this every session
// would break a quarter of an hour after signing in.
func withBkn(w http.ResponseWriter, r *http.Request, fn func(token string) error) {
	id, sess, err := sessions.get(r)
	if err != nil {
		writeAPIErr(w, err)
		return
	}
	err = fn(sess.Access)
	if !errors.Is(err, errUnauthorized) {
		if err != nil {
			writeAPIErr(w, err)
		}
		return
	}
	if sess.Refresh == "" {
		sessions.drop(w, r)
		writeAPIErr(w, errNoSession)
		return
	}
	fresh, rerr := bkn.Refresh(sess.Refresh)
	if rerr != nil {
		sessions.drop(w, r)
		writeAPIErr(w, errNoSession)
		return
	}
	sessions.update(id, func(s *session) {
		s.Access = fresh.Access
		if fresh.Refresh != "" {
			s.Refresh = fresh.Refresh // bkn rotates refresh tokens on every use
		}
	})
	if err := fn(fresh.Access); err != nil {
		writeAPIErr(w, err)
	}
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false})
		return
	}
	ip := clientIP(r)
	if ok, retry := loginByIP.allow(ip); !ok {
		tooMany(w, retry, "too many sign-in attempts from this address")
		return
	}
	var body struct{ Email, Password string }
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		writeAPIErr(w, &apiError{Status: 400, Type: "validation_error", Message: "body must be JSON"})
		return
	}
	account := strings.ToLower(strings.TrimSpace(body.Email))
	if ok, retry := loginByAccount.allow(account); !ok {
		tooMany(w, retry, "too many sign-in attempts for this account")
		return
	}
	toks, email, err := bkn.Login(strings.TrimSpace(body.Email), body.Password)
	if err != nil {
		writeAPIErr(w, err)
		return
	}
	// Signing in successfully clears the counters: a person who mistyped twice
	// and then got it right should not be one typo from a lockout.
	loginByIP.forget(ip)
	loginByAccount.forget(account)
	sessions.create(w, r, &session{Email: email, Access: toks.Access, Refresh: toks.Refresh})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "email": email})
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	sessions.drop(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func handleMe(w http.ResponseWriter, r *http.Request) {
	_, sess, err := sessions.get(r)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "signed_in": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "signed_in": true, "email": sess.Email, "bkn": bkn.Base,
		"max_upload_bytes": MaxUpload,
	})
}

// handleDrive proxies one drive op. The op whitelist is not paranoia about the
// browser -- it is so that a typo returns a clear error here rather than a
// goja stack trace from the far side.
var driveOps = map[string]bool{
	"ls": true, "mkdir": true, "stat": true, "rm": true, "mv": true,
	"quota": true, "share": true, "unshare": true, "shares": true,
	"groups": true, "group-create": true, "group-add": true, "group-remove": true,
}

func handleDrive(w http.ResponseWriter, r *http.Request) {
	var input map[string]any
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&input); err != nil {
		writeAPIErr(w, &apiError{Status: 400, Type: "validation_error", Message: "body must be JSON"})
		return
	}
	op, _ := input["op"].(string)
	if !driveOps[op] {
		writeAPIErr(w, &apiError{Status: 400, Type: "validation_error",
			Message: fmt.Sprintf("op %q is not available through this UI", op)})
		return
	}
	withBkn(w, r, func(token string) error {
		value, err := bkn.Drive(token, input)
		if err != nil {
			return err
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "value": json.RawMessage(value)})
		return nil
	})
}

// handleUpload takes a multipart form and hands the file to the upload hook.
//
// The browser streams a real file; bkn's hook takes base64 in JSON. This is
// where those two meet, and it is the reason for the size cap: the whole file
// exists in memory here, once as bytes and once as base64.
func handleUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(4 << 20); err != nil {
		writeAPIErr(w, &apiError{Status: 400, Type: "validation_error", Message: "expected a multipart upload"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeAPIErr(w, &apiError{Status: 400, Type: "validation_error", Message: "no file in the form"})
		return
	}
	defer file.Close()

	if header.Size > MaxUpload {
		writeAPIErr(w, &apiError{Status: 413, Type: "too_large",
			Message: fmt.Sprintf("%s is %s, over the %s limit",
				header.Filename, humanBytes(header.Size), humanBytes(MaxUpload))})
		return
	}
	raw, err := io.ReadAll(io.LimitReader(file, MaxUpload+1))
	if err != nil {
		writeAPIErr(w, err)
		return
	}
	if len(raw) > MaxUpload {
		writeAPIErr(w, &apiError{Status: 413, Type: "too_large",
			Message: "over the " + humanBytes(MaxUpload) + " limit"})
		return
	}

	name := header.Filename
	if v := r.FormValue("name"); v != "" {
		name = v
	}
	body := map[string]any{
		"drive":          valueOr(r.FormValue("drive"), "user:me"),
		"path":           valueOr(r.FormValue("path"), "/"),
		"name":           name,
		"content_base64": base64.StdEncoding.EncodeToString(raw),
		"content_type":   header.Header.Get("Content-Type"),
	}
	withBkn(w, r, func(token string) error {
		out, err := bkn.Upload(token, body)
		if err != nil {
			return err
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(out)
		return nil
	})
}

// handleDownload asks the drive for a signed URL and sends the browser there,
// so the bytes go straight from bkn to the person and never through this
// process.
func handleDownload(w http.ResponseWriter, r *http.Request) {
	drive := valueOr(r.URL.Query().Get("drive"), "user:me")
	path := r.URL.Query().Get("path")
	if path == "" {
		writeAPIErr(w, &apiError{Status: 400, Type: "validation_error", Message: "path is required"})
		return
	}
	withBkn(w, r, func(token string) error {
		value, err := bkn.Drive(token, map[string]any{
			"op": "download", "drive": drive, "path": path, "ttl": "10m",
		})
		if err != nil {
			return err
		}
		var doc struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(value, &doc); err != nil || doc.URL == "" {
			return &apiError{Status: 502, Type: "bkn_error", Message: "the drive returned no download url"}
		}
		http.Redirect(w, r, bkn.SignedURL(doc.URL), http.StatusFound)
		return nil
	})
}

func valueOr(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
