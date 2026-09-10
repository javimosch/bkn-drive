package main

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// Auto-login: a bookmarkable link that signs you in without a form.
//
// The link IS the credential. That is the whole design, and it is why a key is
// mandatory: an auto-login with no key on a public URL does not mean
// "convenient", it means the drive is readable and writable by anyone who
// finds the hostname. This refuses to enable rather than do that quietly.
//
//	BKN_DRIVE_AUTOLOGIN_EMAIL=admin@vdb.com
//	BKN_DRIVE_AUTOLOGIN_PASSWORD_FILE=/home/dk1/bkn-drive/autologin.pw
//	BKN_DRIVE_AUTOLOGIN_KEY=<32+ random characters>
//
// Then https://host/?k=<key> mints a 30-day session and redirects to /.
type autoLogin struct {
	Email    string
	Password string
	Key      string
}

// MinKeyLen is the shortest key accepted. A guessable key is the same failure
// as no key, arrived at more slowly.
const MinKeyLen = 24

var auto *autoLogin

// loadAutoLogin reads the configuration, reporting why it is off when it is.
// Silence here would be indistinguishable from "enabled and working".
func loadAutoLogin() (*autoLogin, string) {
	email := os.Getenv("BKN_DRIVE_AUTOLOGIN_EMAIL")
	if email == "" {
		return nil, ""
	}

	pwFile := os.Getenv("BKN_DRIVE_AUTOLOGIN_PASSWORD_FILE")
	if pwFile == "" {
		return nil, "auto-login needs BKN_DRIVE_AUTOLOGIN_PASSWORD_FILE"
	}
	raw, err := os.ReadFile(pwFile)
	if err != nil {
		return nil, "auto-login password file unreadable: " + err.Error()
	}
	// A password file the world can read is a password the world knows.
	if info, err := os.Stat(pwFile); err == nil && info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Sprintf("auto-login password file %s is mode %o; it must be 600", pwFile, info.Mode().Perm())
	}
	password := strings.TrimRight(string(raw), "\r\n")
	if password == "" {
		return nil, "auto-login password file is empty"
	}

	key := os.Getenv("BKN_DRIVE_AUTOLOGIN_KEY")
	if len(key) < MinKeyLen {
		return nil, fmt.Sprintf("auto-login needs BKN_DRIVE_AUTOLOGIN_KEY of at least %d characters; refusing to serve an unguarded drive", MinKeyLen)
	}

	return &autoLogin{Email: email, Password: password, Key: key}, ""
}

// handleAutoLogin serves "/". With a ?k= it tries to sign in; without one it
// falls through to the UI, which shows the normal form.
func handleAutoLogin(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		supplied := r.URL.Query().Get("k")
		if supplied == "" || auto == nil || r.URL.Path != "/" {
			next.ServeHTTP(w, r)
			return
		}

		// Brute-forcing the key is the attack this endpoint invites, so it
		// shares the sign-in limiter rather than getting a free budget.
		ip := clientIP(r)
		if ok, retry := loginByIP.allow(ip); !ok {
			tooMany(w, retry, "too many sign-in attempts from this address")
			return
		}

		// Constant time: a key compared byte by byte leaks its prefix through
		// timing, and this one is guessable in a way a password is not --
		// there is no account lockout behind it.
		if subtle.ConstantTimeCompare([]byte(supplied), []byte(auto.Key)) != 1 {
			// Indistinguishable from a plain visit: a wrong key must not
			// confirm that auto-login exists here at all.
			next.ServeHTTP(w, r)
			return
		}

		toks, email, err := bkn.Login(auto.Email, auto.Password)
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		loginByIP.forget(ip)
		sessions.create(w, r, &session{Email: email, Access: toks.Access, Refresh: toks.Refresh}, true)

		// Redirect to a bare "/" so the key leaves the address bar, and does
		// not end up in a screenshot, a bookmark sync, or a referrer header.
		http.Redirect(w, r, "/", http.StatusFound)
	}
}
