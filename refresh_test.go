package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// A fake bkn with the property that matters: its refresh token is
// single-use, because the real one revokes the old session as it issues the
// new one. Presenting a spent token is an error, not a no-op.
type fakeBkn struct {
	mu           sync.Mutex
	validAccess  string
	validRefresh string
	refreshes    int32
	spent        int32
}

func (f *fakeBkn) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/auth/refresh" {
			var body struct {
				RefreshToken string `json:"refresh_token"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)

			f.mu.Lock()
			defer f.mu.Unlock()
			if body.RefreshToken != f.validRefresh {
				atomic.AddInt32(&f.spent, 1)
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"ok":false,"error":{"type":"auth","message":"unknown refresh token"}}`))
				return
			}
			atomic.AddInt32(&f.refreshes, 1)
			f.validAccess = "access-" + time.Now().Format("150405.000000000")
			f.validRefresh = "refresh-" + f.validAccess
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"tokens": map[string]any{
					"access_token": f.validAccess, "refresh_token": f.validRefresh,
				},
			})
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
}

// Two concurrent calls that both find an expired access token must produce
// exactly ONE refresh and leave the session alive. Before the refresh lock,
// the loser presented a spent token, and losing that race signed the person
// out -- which the UI triggered every fifteen minutes by loading a listing and
// a quota at the same time.
func TestConcurrentExpiryRefreshesOnceAndKeepsTheSession(t *testing.T) {
	f := &fakeBkn{validAccess: "access-new", validRefresh: "refresh-1"}
	srv := httptest.NewServer(f.handler())
	defer srv.Close()

	dir := t.TempDir()
	t.Setenv("BKN_DRIVE_STATE", filepath.Join(dir, "sessions.json"))
	bkn = newBkn(srv.URL, "")
	sessions = newSessions(time.Hour)

	rec := httptest.NewRecorder()
	sessions.create(rec, httptest.NewRequest("POST", "/", nil),
		&session{Email: "a@b.c", Access: "access-expired", Refresh: "refresh-1"}, true)
	cookie := rec.Result().Cookies()[0]

	const callers = 8
	var wg sync.WaitGroup
	var ok, failed int32

	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest("POST", "/api/drive", nil)
			r.AddCookie(cookie)
			w := httptest.NewRecorder()

			withBkn(w, r, func(token string) error {
				// Every caller starts holding the stale token.
				if token == "access-expired" {
					return errUnauthorized
				}
				return nil
			})
			if w.Code == http.StatusOK || w.Code == 200 {
				atomic.AddInt32(&ok, 1)
			} else {
				atomic.AddInt32(&failed, 1)
			}
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&f.refreshes); got != 1 {
		t.Errorf("refreshes = %d, want exactly 1", got)
	}
	if got := atomic.LoadInt32(&f.spent); got != 0 {
		t.Errorf("%d callers presented an already-spent refresh token, want 0", got)
	}
	if failed != 0 {
		t.Errorf("%d of %d callers were refused, want 0", failed, callers)
	}
	r := httptest.NewRequest("GET", "/api/me", nil)
	r.AddCookie(cookie)
	if _, _, err := sessions.get(r); err != nil {
		t.Fatalf("the session was dropped: %v", err)
	}
}
