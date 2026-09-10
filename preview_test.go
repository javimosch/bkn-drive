package main

import "testing"

func TestPreviewKind(t *testing.T) {
	cases := []struct{ name, ct, want string }{
		{"logo.png", "", "image"},
		{"clip.MP4", "", "video"},
		{"talk.m4a", "", "audio"},
		{"statuts.pdf", "", "pdf"},
		{"notes.md", "", "text"},
		{"budget.xlsx", "", "none"},
		{"minutes.docx", "application/msword", "none"},
		{"noext", "image/webp", "image"},
		{"data", "application/json", "text"},
		{"mystery", "application/octet-stream", "none"},
	}
	for _, c := range cases {
		if got := previewKind(c.name, c.ct); got != c.want {
			t.Errorf("previewKind(%q, %q) = %q, want %q", c.name, c.ct, got, c.want)
		}
	}
}
