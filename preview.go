package main

import (
	"encoding/json"
	"io"
	"net/http"
	"path"
	"strings"
	"unicode/utf8"
)

// PreviewTextMax bounds an inline text preview. A drive holds whatever people
// put in it, including a 200MB log, and nobody reads that in a modal.
const PreviewTextMax = 512 << 10

// previewKind decides how the browser should show a file, from its name and
// content type.
//
// The browser can embed media and PDFs cross-origin by itself, so those only
// need the download URL. Text cannot be fetched cross-origin without CORS
// headers bkn does not send, which is why this endpoint exists at all.
func previewKind(name, contentType string) string {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))
	ct := strings.ToLower(contentType)

	switch ext {
	case "png", "jpg", "jpeg", "gif", "webp", "avif", "bmp", "ico", "svg":
		return "image"
	case "mp4", "webm", "ogv", "mov", "m4v":
		return "video"
	case "mp3", "wav", "ogg", "oga", "m4a", "flac", "aac", "opus":
		return "audio"
	case "pdf":
		return "pdf"
	case "txt", "md", "markdown", "csv", "tsv", "log", "json", "yml", "yaml",
		"toml", "ini", "conf", "sh", "bash", "zsh", "js", "jsx", "ts", "tsx",
		"go", "py", "rb", "rs", "c", "h", "cpp", "java", "php", "sql", "css",
		"html", "xml", "env", "gitignore", "dockerfile", "makefile":
		return "text"
	}

	switch {
	case strings.HasPrefix(ct, "image/"):
		return "image"
	case strings.HasPrefix(ct, "video/"):
		return "video"
	case strings.HasPrefix(ct, "audio/"):
		return "audio"
	case ct == "application/pdf":
		return "pdf"
	case strings.HasPrefix(ct, "text/"),
		ct == "application/json", ct == "application/xml":
		return "text"
	}

	// Office formats land here. There is no honest way to render a .docx in a
	// browser without shipping it to a third-party viewer, which is not a
	// thing to do quietly with an association's documents.
	return "none"
}

// handlePreview returns what the UI needs to show a file inline. For text it
// includes the content, fetched through this process because bkn's signed URL
// is on another origin and sends no CORS headers.
func handlePreview(w http.ResponseWriter, r *http.Request) {
	drive := valueOr(r.URL.Query().Get("drive"), "user:me")
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		writeAPIErr(w, &apiError{Status: 400, Type: "validation_error", Message: "path is required"})
		return
	}

	withBkn(w, r, func(token string) error {
		value, err := bkn.Drive(token, map[string]any{
			"op": "download", "drive": drive, "path": filePath, "ttl": "10m",
		})
		if err != nil {
			return err
		}
		var doc struct {
			URL         string `json:"url"`
			Size        int64  `json:"size"`
			ContentType string `json:"content_type"`
		}
		if err := json.Unmarshal(value, &doc); err != nil || doc.URL == "" {
			return &apiError{Status: 502, Type: "bkn_error", Message: "the drive returned no download url"}
		}

		name := path.Base(filePath)
		kind := previewKind(name, doc.ContentType)
		out := map[string]any{
			"ok": true, "kind": kind, "name": name,
			"size": doc.Size, "content_type": doc.ContentType,
		}

		if kind != "text" {
			// Media and PDFs embed straight from bkn; nothing to proxy.
			writeJSON(w, http.StatusOK, out)
			return nil
		}

		if doc.Size > PreviewTextMax {
			out["kind"] = "too_big"
			out["limit"] = PreviewTextMax
			writeJSON(w, http.StatusOK, out)
			return nil
		}

		resp, err := bkn.HTTP.Get(bkn.SignedURL(doc.URL))
		if err != nil {
			return &apiError{Status: 502, Type: "unreachable", Message: "could not read the file: " + err.Error()}
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(resp.Body, PreviewTextMax+1))
		if err != nil {
			return &apiError{Status: 502, Type: "bkn_error", Message: "could not read the file: " + err.Error()}
		}
		// A file with a .txt name is not necessarily text. Rendering binary as
		// text produces a screenful of replacement characters, so say so
		// instead.
		if !utf8.Valid(raw) {
			out["kind"] = "none"
			writeJSON(w, http.StatusOK, out)
			return nil
		}
		out["text"] = string(raw)
		out["truncated"] = len(raw) > PreviewTextMax
		writeJSON(w, http.StatusOK, out)
		return nil
	})
}
