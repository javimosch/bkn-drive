package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLimiterAllowsUpToTheLimitThenRefuses(t *testing.T) {
	l := newLimiter(3, time.Minute)
	for i := 1; i <= 3; i++ {
		if ok, _ := l.allow("k"); !ok {
			t.Fatalf("attempt %d refused, want allowed", i)
		}
	}
	ok, retry := l.allow("k")
	if ok {
		t.Fatal("the fourth attempt was allowed, want refused")
	}
	if retry <= 0 || retry > time.Minute {
		t.Fatalf("retry = %v, want a positive duration inside the window", retry)
	}
}

func TestLimiterIsPerKey(t *testing.T) {
	l := newLimiter(1, time.Minute)
	l.allow("a")
	if ok, _ := l.allow("b"); !ok {
		t.Fatal("one key's attempts blocked another key")
	}
}

// A window that never expires is a permanent lockout.
func TestLimiterForgetsOldAttempts(t *testing.T) {
	l := newLimiter(2, 40*time.Millisecond)
	l.allow("k")
	l.allow("k")
	if ok, _ := l.allow("k"); ok {
		t.Fatal("over the limit but allowed")
	}
	time.Sleep(60 * time.Millisecond)
	if ok, _ := l.allow("k"); !ok {
		t.Fatal("still blocked after the window passed")
	}
}

func TestForgetClearsTheCounter(t *testing.T) {
	l := newLimiter(1, time.Minute)
	l.allow("k")
	l.forget("k")
	if ok, _ := l.allow("k"); !ok {
		t.Fatal("forget did not clear the counter")
	}
}

// The leftmost X-Forwarded-For entry is whatever the client chose to send, so
// trusting it lets anyone reset their own rate limit by inventing an address.
func TestClientIPUsesTheLastForwardedEntry(t *testing.T) {
	r, _ := http.NewRequest("POST", "/api/login", nil)
	r.RemoteAddr = "127.0.0.1:55000"
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 9.9.9.9")
	if got := clientIP(r); got != "9.9.9.9" {
		t.Fatalf("clientIP = %q, want 9.9.9.9 (the address the proxy saw)", got)
	}
}

// Off loopback the header is not ours and must be ignored entirely.
func TestClientIPIgnoresForwardedFromANonLoopbackPeer(t *testing.T) {
	r, _ := http.NewRequest("POST", "/api/login", nil)
	r.RemoteAddr = "203.0.113.7:44000"
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := clientIP(r); got != "203.0.113.7" {
		t.Fatalf("clientIP = %q, want the real peer", got)
	}
}

// The reset endpoint must be invisible from outside. Traefik always sets
// X-Forwarded-For, so a request carrying one came through the proxy however
// loopback-looking its peer address is.
func TestResetLimitsIsLoopbackOnly(t *testing.T) {
	cases := []struct {
		name, remote, fwd string
		wantStatus        int
	}{
		{"direct loopback", "127.0.0.1:5000", "", 200},
		{"through traefik", "127.0.0.1:5000", "203.0.113.9", 404},
		{"remote peer", "203.0.113.9:5000", "", 404},
	}
	for _, c := range cases {
		r := httptest.NewRequest("POST", "/api/_reset-limits", nil)
		r.RemoteAddr = c.remote
		if c.fwd != "" {
			r.Header.Set("X-Forwarded-For", c.fwd)
		}
		w := httptest.NewRecorder()
		handleResetLimits(w, r)
		if w.Code != c.wantStatus {
			t.Errorf("%s: status = %d, want %d", c.name, w.Code, c.wantStatus)
		}
	}
}

func TestResetActuallyClears(t *testing.T) {
	l := newLimiter(1, time.Hour)
	l.allow("k")
	if ok, _ := l.allow("k"); ok {
		t.Fatal("limit not enforced")
	}
	l.reset()
	if ok, _ := l.allow("k"); !ok {
		t.Fatal("reset did not clear the limiter")
	}
}
