package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The whole point of persisting sessions is that a deploy does not sign people
// out. If this breaks, "stay signed in" quietly becomes "signed in until the
// next release", which is exactly the failure it was added to fix.
func TestASessionSurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BKN_DRIVE_STATE", filepath.Join(dir, "sessions.json"))

	first := newSessions(12 * time.Hour)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/login", nil)
	first.create(w, r, &session{Email: "a@b.c", Access: "acc", Refresh: "ref"}, true)

	cookie := w.Result().Cookies()[0]
	if cookie.MaxAge < int(LongTTL.Seconds()) {
		t.Fatalf("remembered cookie MaxAge = %d, want >= %d", cookie.MaxAge, int(LongTTL.Seconds()))
	}

	// A second store is what the next process sees.
	second := newSessions(12 * time.Hour)
	r2 := httptest.NewRequest("GET", "/api/me", nil)
	r2.AddCookie(cookie)
	_, sess, err := second.get(r2)
	if err != nil {
		t.Fatalf("session did not survive: %v", err)
	}
	if sess.Email != "a@b.c" || sess.Refresh != "ref" {
		t.Fatalf("session came back wrong: %+v", sess)
	}
}

func TestAnExpiredSessionIsNotRestored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions.json")
	t.Setenv("BKN_DRIVE_STATE", path)
	if err := os.WriteFile(path,
		[]byte(`{"old":{"email":"a@b.c","refresh":"r","expires":"2020-01-01T00:00:00Z"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s := newSessions(time.Hour)
	if len(s.byID) != 0 {
		t.Fatalf("restored %d expired sessions, want 0", len(s.byID))
	}
}

// Refresh tokens are 30-day keys. The file must not be group or world readable.
func TestTheStateFileIsPrivate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "sessions.json")
	t.Setenv("BKN_DRIVE_STATE", path)
	s := newSessions(time.Hour)
	w := httptest.NewRecorder()
	s.create(w, httptest.NewRequest("POST", "/", nil), &session{Email: "a@b.c"}, false)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("no state file: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("state file mode = %o, want 600", mode)
	}
}
