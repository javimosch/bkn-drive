package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// A share page must not be cacheable: it is addressed by a secret and it
// contains a signed, short-lived download URL.
func TestSharePageIsNotCacheable(t *testing.T) {
	w := httptest.NewRecorder()
	sharePage(w, 200, "x", "<h1>x</h1>")
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if got := w.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("Referrer-Policy = %q, want no-referrer", got)
	}
}

// A file name is chosen by whoever uploaded it, so it reaches this page as
// untrusted text.
func TestSharePageEscapesTheFileName(t *testing.T) {
	w := httptest.NewRecorder()
	sharePage(w, 200, `<script>alert(1)</script>`, "<h1>ok</h1>")
	body := w.Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatal("the title was not escaped")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Fatal("expected the escaped form in the title")
	}
}
