package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"sync"
	"time"
)

// sessions map an opaque cookie to one person's bkn tokens.
//
// In memory on purpose: a restart signs everyone out, which for a drive UI is
// the right trade. Persisting them would mean writing refresh tokens to disk,
// and a refresh token is a 30-day key to somebody's files.
type session struct {
	Email    string
	Sub      string
	Access   string
	Refresh  string
	LastSeen time.Time
}

type sessionStore struct {
	mu   sync.Mutex
	byID map[string]*session
	ttl  time.Duration
}

var errNoSession = errors.New("not signed in")

const sessionCookie = "bd_session"

func newSessions(ttl time.Duration) *sessionStore {
	s := &sessionStore{byID: map[string]*session{}, ttl: ttl}
	go s.sweep()
	return s
}

// sweep drops idle sessions. Without it a long-lived server accumulates
// refresh tokens for people who closed the tab weeks ago.
func (s *sessionStore) sweep() {
	for range time.Tick(10 * time.Minute) {
		cut := time.Now().Add(-s.ttl)
		s.mu.Lock()
		for id, sess := range s.byID {
			if sess.LastSeen.Before(cut) {
				delete(s.byID, id)
			}
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

func (s *sessionStore) create(w http.ResponseWriter, r *http.Request, sess *session) {
	id := newID()
	sess.LastSeen = time.Now()
	s.mu.Lock()
	s.byID[id] = sess
	s.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    id,
		Path:     "/",
		HttpOnly: true, // the browser may send it, never read it
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		MaxAge:   int(s.ttl.Seconds()),
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
	sess.LastSeen = time.Now()
	return ck.Value, sess, nil
}

func (s *sessionStore) update(id string, fn func(*session)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.byID[id]; ok {
		fn(sess)
	}
}

func (s *sessionStore) drop(w http.ResponseWriter, r *http.Request) {
	if ck, err := r.Cookie(sessionCookie); err == nil {
		s.mu.Lock()
		delete(s.byID, ck.Value)
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, MaxAge: -1,
	})
}
