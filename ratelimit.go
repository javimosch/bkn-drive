package main

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// limiter is a fixed-window counter per key.
//
// Deliberately not a token bucket: for a login endpoint the useful property is
// "no more than N attempts in the last W", which a window states directly and
// an operator can reason about without simulating a refill rate.
type limiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
}

func newLimiter(limit int, window time.Duration) *limiter {
	l := &limiter{hits: map[string][]time.Time{}, limit: limit, window: window}
	go l.sweep()
	return l
}

// sweep keeps the map from growing without bound. Without it, a slow scan
// across many source addresses is a memory leak with extra steps.
func (l *limiter) sweep() {
	for range time.Tick(5 * time.Minute) {
		l.mu.Lock()
		cut := time.Now().Add(-l.window)
		for key, times := range l.hits {
			kept := times[:0]
			for _, t := range times {
				if t.After(cut) {
					kept = append(kept, t)
				}
			}
			if len(kept) == 0 {
				delete(l.hits, key)
			} else {
				l.hits[key] = kept
			}
		}
		l.mu.Unlock()
	}
}

// allow records an attempt and reports whether it is within the limit, plus
// how long until the oldest attempt in the window expires.
func (l *limiter) allow(key string) (bool, time.Duration) {
	now := time.Now()
	cut := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	times := l.hits[key]
	kept := times[:0]
	for _, t := range times {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}

	if len(kept) >= l.limit {
		l.hits[key] = kept
		return false, time.Until(kept[0].Add(l.window))
	}
	l.hits[key] = append(kept, now)
	return true, 0
}

// forget clears a key. A successful sign-in should not leave the person one
// typo away from being locked out.
func (l *limiter) forget(key string) {
	l.mu.Lock()
	delete(l.hits, key)
	l.mu.Unlock()
}

// clientIP resolves who is calling, from behind traefik.
//
// X-Forwarded-For is only consulted when the direct peer is loopback, because
// that is the only case where a proxy we control produced it -- and the LAST
// entry is used, not the first. A client can send its own X-Forwarded-For and
// traefik appends to it, so the leftmost value is attacker-controlled and the
// rightmost is the address traefik actually saw.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return host
	}
	fwd := r.Header.Get("X-Forwarded-For")
	if fwd == "" {
		return host
	}
	parts := strings.Split(fwd, ",")
	return strings.TrimSpace(parts[len(parts)-1])
}

func tooMany(w http.ResponseWriter, retry time.Duration, what string) {
	secs := int(retry.Seconds())
	if secs < 1 {
		secs = 1
	}
	w.Header().Set("Retry-After", itoa(secs))
	writeJSON(w, http.StatusTooManyRequests, map[string]any{
		"ok": false, "error": map[string]any{
			"type": "rate_limited", "recoverable": true,
			"message": what + " -- try again in " + humanDuration(retry),
		}})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func humanDuration(d time.Duration) string {
	if d < time.Minute {
		secs := int(d.Seconds())
		if secs < 1 {
			secs = 1
		}
		return itoa(secs) + "s"
	}
	return itoa(int(d.Minutes())+1) + " min"
}

// throttled caps how fast one address can drive the authenticated API.
func throttled(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ok, retry := apiByIP.allow(clientIP(r)); !ok {
			tooMany(w, retry, "too many requests")
			return
		}
		next(w, r)
	}
}
