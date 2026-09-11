package main

import "testing"

// bkn reports a thrown script error as HTTP 422 whose body is the entire run
// record. Every drive refusal -- quota exceeded, name taken, no access --
// arrives this way, so failing to unwrap it replaces every useful message in
// the UI with a wall of JSON.
func TestDecodeErrUnwrapsAThrownScriptError(t *testing.T) {
	body := []byte(`{"ok":false,"run":{"id":"01X","name":"drive","status":"error",` +
		`"error":"Error: {\"error\":\"/documents already exists in user:01X\"} at fail (<eval>:35:9(15))"}}`)
	got := decodeErr(422, body)
	want := "/documents already exists in user:01X"
	if got.Message != want {
		t.Fatalf("message = %q, want %q", got.Message, want)
	}
	if got.Type != "drive_error" {
		t.Fatalf("type = %q, want drive_error", got.Type)
	}
}

func TestDecodeErrStillHandlesBknsOwnEnvelope(t *testing.T) {
	got := decodeErr(404, []byte(`{"ok":false,"error":{"type":"not_found","message":"no such hook"}}`))
	if got.Message != "no such hook" || got.Type != "not_found" {
		t.Fatalf("got %+v", got)
	}
}

func TestDecodeErrHandlesAHookReply(t *testing.T) {
	got := decodeErr(413, []byte(`{"ok":false,"error":"drive quota exceeded: 20 bytes","field":"content_base64"}`))
	if got.Message != "drive quota exceeded: 20 bytes" {
		t.Fatalf("got %+v", got)
	}
}

func TestUnwrapThrowKeepsPlainMessages(t *testing.T) {
	if got := unwrapThrow("boom"); got != "boom" {
		t.Fatalf("got %q", got)
	}
	if got := unwrapThrow(""); got == "" {
		t.Fatal("an empty error should still say something")
	}
}

// A download URL is handed to the browser; a preview fetch is made by this
// process. When bkn is on localhost and the browser is not, those cannot be
// the same address -- getting it wrong sends the reader to 127.0.0.1.
func TestSignedURLUsesThePublicBaseAndFetchUsesTheDialledOne(t *testing.T) {
	c := newBkn("http://127.0.0.1:8804", "https://bkn.example.org")

	if got := c.SignedURL("/v1/files/ns/name?sig=x"); got != "https://bkn.example.org/v1/files/ns/name?sig=x" {
		t.Errorf("SignedURL = %q, want the public base", got)
	}
	if got := c.fetchURL("/v1/files/ns/name?sig=x"); got != "http://127.0.0.1:8804/v1/files/ns/name?sig=x" {
		t.Errorf("fetchURL = %q, want the dialled base", got)
	}
}

func TestPublicBaseFallsBackToTheDialledBase(t *testing.T) {
	c := newBkn("https://bkn.example.org", "")
	if c.Public != "https://bkn.example.org" {
		t.Fatalf("Public = %q, want the base when no public base is set", c.Public)
	}
}
