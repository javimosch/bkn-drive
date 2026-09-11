package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
)

// Share links: /s/<token>, for someone with no account and no session.
//
// Rendered server-side as one self-contained page rather than the React app.
// The recipient is a notary or a member opening a link from an email; loading
// a single-page application, three CDN scripts and a session cookie to show
// them one download button would be worse in every way.

const shareBase = `<!doctype html>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>%s</title>
<style>
 body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;
      background:#f7f8fa;color:#16181d;
      font-family:ui-sans-serif,-apple-system,"Segoe UI",Roboto,Arial,sans-serif}
 .card{background:#fff;border:1px solid #e8e9ee;border-radius:14px;padding:28px;
       width:100%%;max-width:26rem;box-shadow:0 1px 2px rgba(16,24,40,.04),0 8px 24px -12px rgba(16,24,40,.12)}
 h1{font-size:1rem;margin:0 0 .35rem}
 p{color:#6b7280;font-size:.875rem;margin:0 0 1.25rem}
 .btn{display:block;text-align:center;text-decoration:none;background:#3b5bdb;color:#fff;
      border:0;border-radius:9px;padding:.7rem 1rem;font-size:.9rem;font-weight:500;
      width:100%%;cursor:pointer}
 .btn:hover{background:#31489f}
 input{width:100%%;box-sizing:border-box;border:1px solid #e8e9ee;border-radius:9px;
       padding:.6rem .75rem;font-size:.9rem;margin-bottom:.9rem}
 .err{color:#c92a2a;font-size:.85rem;margin:0 0 .9rem}
 .meta{color:#9ca3af;font-size:.75rem;margin-top:1rem;text-align:center}
</style>
<div class="card">%s</div>`

func sharePage(w http.ResponseWriter, status int, title, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// A share page must never be cached by a proxy: the download URL inside it
	// is signed and short-lived, and the page itself is addressed by a secret.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.WriteHeader(status)
	fmt.Fprintf(w, shareBase, html.EscapeString(title), body)
}

type linkResult struct {
	OK               bool   `json:"ok"`
	Name             string `json:"name"`
	Size             int64  `json:"size"`
	ContentType      string `json:"content_type"`
	URL              string `json:"url"`
	PasswordRequired bool   `json:"password_required"`
	Error            string `json:"error"`
}

// resolveLink asks the public drive-link hook. This process has no privileged
// path to a link either -- it holds the same token the visitor typed.
func (c *bknClient) resolveLink(token, password string) (*linkResult, int, error) {
	payload, _ := json.Marshal(map[string]string{"token": token, "password": password})
	resp, err := c.HTTP.Post(c.Base+"/v1/hooks/drive-link", "application/json", strings.NewReader(string(payload)))
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	var out linkResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("the drive returned an unreadable answer")
	}
	return &out, resp.StatusCode, nil
}

func handleShare(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.URL.Path, "/s/")
	if token == "" || strings.Contains(token, "/") {
		sharePage(w, http.StatusNotFound, "Not found",
			`<h1>This link is not valid</h1><p>Check that it was copied in full.</p>`)
		return
	}

	password := ""
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		password = r.FormValue("password")
	}

	res, status, err := bkn.resolveLink(token, password)
	if err != nil {
		sharePage(w, http.StatusBadGateway, "Unavailable",
			`<h1>The drive is not reachable</h1><p>Try again in a moment.</p>`)
		return
	}

	switch {
	case res.OK:
		sharePage(w, http.StatusOK, res.Name, fmt.Sprintf(
			`<h1>%s</h1><p>%s · shared from a drive</p>
			 <a class="btn" href="%s" download>Download</a>
			 <p class="meta">This download link is valid for a few minutes.</p>`,
			html.EscapeString(res.Name), html.EscapeString(humanBytes(res.Size)),
			html.EscapeString(bkn.SignedURL(res.URL))))

	case res.PasswordRequired:
		errLine := ""
		if res.Error != "" {
			errLine = `<p class="err">` + html.EscapeString(res.Error) + `</p>`
		}
		name := "Protected file"
		if res.Name != "" {
			name = res.Name
		}
		sharePage(w, http.StatusUnauthorized, name, fmt.Sprintf(
			`<h1>This file is password protected</h1>
			 <p>Enter the password you were given.</p>%s
			 <form method="post"><input type="password" name="password" autofocus
			  placeholder="Password" autocomplete="off">
			 <button class="btn" type="submit">Open</button></form>`, errLine))

	case status == http.StatusTooManyRequests:
		sharePage(w, http.StatusTooManyRequests, "Too many attempts",
			`<h1>Too many attempts</h1><p>Wait a minute and try again.</p>`)

	default:
		msg := res.Error
		if msg == "" {
			msg = "this link is not valid"
		}
		sharePage(w, status, "Not available",
			`<h1>`+html.EscapeString(strings.ToUpper(msg[:1])+msg[1:])+`</h1>
			 <p>Ask whoever shared it for a new link.</p>`)
	}
}
