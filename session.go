package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// sessions map an opaque cookie to one person's bkn tokens.
//
// They are written to disk, at 0600, so that a deploy does not sign everyone
// out. That is a deliberate reversal of the original design: a refresh token
// is a 30-day key to somebody's files, and keeping it only in memory was
// safer. But "auto-login" and "restarting logs you out" cannot both be true,
// and for a drive people keep open in a tab, being signed out by an unrelated
// deploy is the failure they actually hit. The file is owned by the service
// user and readable by nobody else; anyone who can read it can already read
// the binary and the environment.
type session struct {
	Email    string    `json:"email"`
	Sub      string    `json:"sub,omitempty"`
	Access   string    `json:"access"`
	Refresh  string    `json:"refresh"`
	LastSeen time.Time `json:"last_seen"`
	// Expires is absolute, so "stay signed in" survives a restart rather than
	// silently reverting to the default idle window.
	Expires time.Time `json:"expires"`
}

type sessionStore struct {
	mu    sync.Mutex
	byID  map[string]*session
	ttl   time.Duration
	long  time.Duration
	path  string
	dirty bool

	// One refresh at a time per session. bkn's refresh tokens are strictly
	// single-use -- Refresh revokes the old session row -- so two requests
	// refreshing at once means the second presents a spent token and fails.
	// Without this, any pair of concurrent calls that outlive the 15-minute
	// access token logs the person out.
	refreshing map[string]*sync.Mutex
}

var errNoSession = errors.New("not signed in")

const sessionCookie = "bd_session"

// LongTTL is how long "stay signed in" lasts. It is capped by bkn's own
// refresh token lifetime (30 days), and every refresh rotates the token, so an
// active tab keeps renewing itself.
const LongTTL = 30 * 24 * time.Hour

func statePath() string {
	if p := os.Getenv("BKN_DRIVE_STATE"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "sessions.json"
	}
	return filepath.Join(home, ".local", "share", "bkn-drive", "sessions.json")
}

func newSessions(ttl time.Duration) *sessionStore {
	s := &sessionStore{
		byID: map[string]*session{}, ttl: ttl, long: LongTTL, path: statePath(),
		refreshing: map[string]*sync.Mutex{},
	}
	s.load()
	go s.sweep()
	return s
}

// load restores sessions from disk, dropping anything already expired.
func (s *sessionStore) load() {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return // no state yet is the normal first run
	}
	var stored map[string]*session
	if err := json.Unmarshal(raw, &stored); err != nil {
		return // a corrupt file signs people out; it does not stop the server
	}
	now := time.Now()
	for id, sess := range stored {
		if sess != nil && sess.Expires.After(now) {
			s.byID[id] = sess
		}
	}
}

// save writes the store. Through a temp file and a rename, so a crash midway
// leaves the previous state rather than a truncated one.
func (s *sessionStore) save() {
	raw, err := json.Marshal(s.byID)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, s.path)
}

// sweep drops idle sessions. Without it a long-lived server accumulates
// refresh tokens for people who closed the tab weeks ago.
func (s *sessionStore) sweep() {
	for range time.Tick(10 * time.Minute) {
		now := time.Now()
		s.mu.Lock()
		changed := false
		for id, sess := range s.byID {
			if sess.Expires.Before(now) {
				delete(s.byID, id)
				changed = true
			}
		}
		if changed || s.dirty {
			s.save()
			s.dirty = false
		}
		s.mu.Unlock()
	}
}

func newID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// A session id that is not random is not a session id.
		panic("crypto/rand unavailable: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func (s *sessionStore) create(w http.ResponseWriter, r *http.Request, sess *session, remember bool) {
	id := newID()
	ttl := s.ttl
	if remember {
		ttl = s.long
	}
	sess.LastSeen = time.Now()
	sess.Expires = time.Now().Add(ttl)

	s.mu.Lock()
	s.byID[id] = sess
	s.save()
	s.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    id,
		Path:     "/",
		HttpOnly: true, // the browser may send it, never read it
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		MaxAge:   int(ttl.Seconds()),
	})
}

func (s *sessionStore) get(r *http.Request) (string, *session, error) {
	ck, err := r.Cookie(sessionCookie)
	if err != nil || ck.Value == "" {
		return "", nil, errNoSession
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.byID[ck.Value]
	if !ok {
		return "", nil, errNoSession
	}
	if !sess.Expires.IsZero() && sess.Expires.Before(time.Now()) {
		delete(s.byID, ck.Value)
		return "", nil, errNoSession
	}
	sess.LastSeen = time.Now()
	return ck.Value, sess, nil
}

// refreshLock returns the lock guarding this session's token rotation.
func (s *sessionStore) refreshLock(id string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refreshing == nil {
		s.refreshing = map[string]*sync.Mutex{}
	}
	mu, ok := s.refreshing[id]
	if !ok {
		mu = &sync.Mutex{}
		s.refreshing[id] = mu
	}
	return mu
}

// peek reads a session by id without touching cookies, so a caller that has
// waited on the refresh lock can see what the winner stored.
func (s *sessionStore) peek(id string) (*session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.byID[id]
	return sess, ok
}

func (s *sessionStore) update(id string, fn func(*session)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.byID[id]; ok {
		fn(sess)
		// A rotated refresh token that is only in memory is lost on restart,
		// and the old one is already spent -- so persist it now, not later.
		s.save()
	}
}

func (s *sessionStore) drop(w http.ResponseWriter, r *http.Request) {
	if ck, err := r.Cookie(sessionCookie); err == nil {
		s.mu.Lock()
		delete(s.byID, ck.Value)
		delete(s.refreshing, ck.Value)
		s.save()
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, MaxAge: -1,
	})
}
