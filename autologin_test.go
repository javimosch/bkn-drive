package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Auto-login without a key is an open drive. It must refuse to enable, and say
// why, rather than starting up looking healthy.
func TestAutoLoginRefusesWithoutAKey(t *testing.T) {
	dir := t.TempDir()
	pw := filepath.Join(dir, "pw")
	if err := os.WriteFile(pw, []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BKN_DRIVE_AUTOLOGIN_EMAIL", "a@b.c")
	t.Setenv("BKN_DRIVE_AUTOLOGIN_PASSWORD_FILE", pw)
	t.Setenv("BKN_DRIVE_AUTOLOGIN_KEY", "")

	cfg, why := loadAutoLogin()
	if cfg != nil {
		t.Fatal("enabled auto-login with no key")
	}
	if why == "" {
		t.Fatal("disabled silently; the operator would never know")
	}
}

func TestAutoLoginRefusesAShortKey(t *testing.T) {
	dir := t.TempDir()
	pw := filepath.Join(dir, "pw")
	os.WriteFile(pw, []byte("secret"), 0o600)
	t.Setenv("BKN_DRIVE_AUTOLOGIN_EMAIL", "a@b.c")
	t.Setenv("BKN_DRIVE_AUTOLOGIN_PASSWORD_FILE", pw)
	t.Setenv("BKN_DRIVE_AUTOLOGIN_KEY", "short")

	if cfg, why := loadAutoLogin(); cfg != nil || why == "" {
		t.Fatalf("accepted a 5-character key (cfg=%v, why=%q)", cfg != nil, why)
	}
}

// A password file others can read is a password others know.
func TestAutoLoginRefusesAWorldReadablePasswordFile(t *testing.T) {
	dir := t.TempDir()
	pw := filepath.Join(dir, "pw")
	os.WriteFile(pw, []byte("secret"), 0o644)
	t.Setenv("BKN_DRIVE_AUTOLOGIN_EMAIL", "a@b.c")
	t.Setenv("BKN_DRIVE_AUTOLOGIN_PASSWORD_FILE", pw)
	t.Setenv("BKN_DRIVE_AUTOLOGIN_KEY", "0123456789012345678901234567")

	cfg, why := loadAutoLogin()
	if cfg != nil {
		t.Fatal("accepted a 644 password file")
	}
	if why == "" {
		t.Fatal("no reason given")
	}
}

func TestAutoLoginAcceptsAProperConfig(t *testing.T) {
	dir := t.TempDir()
	pw := filepath.Join(dir, "pw")
	os.WriteFile(pw, []byte("secret\n"), 0o600)
	t.Setenv("BKN_DRIVE_AUTOLOGIN_EMAIL", "a@b.c")
	t.Setenv("BKN_DRIVE_AUTOLOGIN_PASSWORD_FILE", pw)
	t.Setenv("BKN_DRIVE_AUTOLOGIN_KEY", "0123456789012345678901234567")

	cfg, why := loadAutoLogin()
	if cfg == nil {
		t.Fatalf("refused a valid config: %s", why)
	}
	if cfg.Password != "secret" {
		t.Fatalf("password = %q, want the file contents with the newline trimmed", cfg.Password)
	}
}

// Unset is off, with nothing to report.
func TestAutoLoginIsOffByDefault(t *testing.T) {
	t.Setenv("BKN_DRIVE_AUTOLOGIN_EMAIL", "")
	if cfg, why := loadAutoLogin(); cfg != nil || why != "" {
		t.Fatalf("cfg=%v why=%q, want off and quiet", cfg != nil, why)
	}
}
