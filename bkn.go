package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// bknClient talks to a bkn instance on behalf of one signed-in person.
//
// Tokens never reach the browser: the UI holds an opaque session cookie and
// this process holds the bearer token. A drive is exactly the kind of thing
// worth that arrangement -- a token in localStorage is one XSS away from
// somebody else's files, and the drive's whole promise is that it is not.
type bknClient struct {
	Base string
	// Public is where the BROWSER should reach bkn. It differs from Base when
	// this process talks to a bkn on localhost but hands out signed download
	// URLs: a URL pointing at 127.0.0.1 is useless to whoever is reading it.
	Public string
	HTTP   *http.Client
}

func newBkn(base, public string) *bknClient {
	if public == "" {
		public = base
	}
	return &bknClient{
		Base:   strings.TrimSuffix(base, "/"),
		Public: strings.TrimSuffix(public, "/"),
		// Uploads are the slow path and 25MB over a domestic uplink is not
		// quick; the default 30s would cut them off mid-flight.
		HTTP: &http.Client{Timeout: 5 * time.Minute},
	}
}

// DefaultBknURL points at a bkn on the same machine. A tool published for
// other people must not default to somebody else's server: whoever runs this
// without setting BKN_URL should reach their own bkn, not mine.
const DefaultBknURL = "http://127.0.0.1:8804"

func bknBase() string {
	if v := os.Getenv("BKN_URL"); v != "" {
		return v
	}
	return DefaultBknURL
}

// bknPublicBase is the address the browser uses. Set it when BKN_URL points at
// a loopback or private address that only this process can reach.
func bknPublicBase() string { return os.Getenv("BKN_PUBLIC_URL") }

// apiError carries bkn's typed error so the UI can show what bkn actually
// said rather than a generic failure.
type apiError struct {
	Status  int
	Type    string
	Message string
}

func (e *apiError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("bkn returned %d", e.Status)
	}
	return e.Message
}

var errUnauthorized = errors.New("unauthorized")

type tokens struct {
	Access  string `json:"access_token"`
	Refresh string `json:"refresh_token"`
}

func (c *bknClient) post(path, token string, body any, out any) error {
	var buf io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		buf = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(http.MethodPost, c.Base+path, buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return &apiError{Status: 0, Type: "unreachable", Message: "bkn is unreachable: " + err.Error()}
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode == http.StatusUnauthorized {
		return errUnauthorized
	}
	if resp.StatusCode >= 400 {
		return decodeErr(resp.StatusCode, raw)
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// decodeErr unwraps both shapes bkn uses: its own {"error":{...}} envelope and
// a hook script's plain {"error":"..."} body.
func decodeErr(status int, raw []byte) *apiError {
	var env struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &env); err == nil && env.Error.Message != "" {
		return &apiError{Status: status, Type: env.Error.Type, Message: env.Error.Message}
	}
	// A thrown script error comes back as HTTP 422 whose body is the whole run
	// record, with the message buried in run.error. Miss this and the UI shows
	// a wall of JSON where a sentence belongs.
	var run struct {
		Run struct {
			Error  string `json:"error"`
			Status string `json:"status"`
		} `json:"run"`
	}
	if err := json.Unmarshal(raw, &run); err == nil && run.Run.Error != "" {
		return &apiError{Status: status, Type: "drive_error", Message: unwrapThrow(run.Run.Error)}
	}
	var flat struct {
		Error string `json:"error"`
		Field string `json:"field"`
	}
	if err := json.Unmarshal(raw, &flat); err == nil && flat.Error != "" {
		return &apiError{Status: status, Type: "validation_error", Message: flat.Error}
	}
	return &apiError{Status: status, Type: "bkn_error", Message: strings.TrimSpace(string(raw))}
}

func (c *bknClient) Login(email, password string) (*tokens, string, error) {
	// The user rides INSIDE tokens, not beside it. Guessing that shape wrong
	// is silent: login still succeeds and the UI just shows a blank name.
	var resp struct {
		Tokens struct {
			tokens
			User struct {
				ID    string `json:"id"`
				Email string `json:"email"`
				Name  string `json:"name"`
			} `json:"user"`
		} `json:"tokens"`
	}
	err := c.post("/v1/auth/login", "", map[string]string{
		"email": email, "password": password,
	}, &resp)
	if errors.Is(err, errUnauthorized) {
		return nil, "", &apiError{Status: 401, Type: "auth", Message: "wrong email or password"}
	}
	if err != nil {
		return nil, "", err
	}
	if resp.Tokens.Access == "" {
		return nil, "", &apiError{Status: 500, Type: "auth", Message: "bkn returned no access token"}
	}
	who := resp.Tokens.User.Email
	if who == "" {
		who = email
	}
	return &tokens{Access: resp.Tokens.Access, Refresh: resp.Tokens.Refresh}, who, nil
}

func (c *bknClient) Refresh(refresh string) (*tokens, error) {
	var resp struct {
		Tokens tokens `json:"tokens"`
	}
	if err := c.post("/v1/auth/refresh", "", map[string]string{"refresh_token": refresh}, &resp); err != nil {
		return nil, err
	}
	if resp.Tokens.Access == "" {
		return nil, &apiError{Status: 500, Type: "auth", Message: "bkn returned no access token"}
	}
	return &resp.Tokens, nil
}

// scriptResult is the envelope a script run returns. A script that throws
// comes back as HTTP 422 with the thrown message in run.error, which is where
// every drive validation failure lands.
type scriptResult struct {
	OK    bool            `json:"ok"`
	Value json.RawMessage `json:"value"`
	Run   struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	} `json:"run"`
}

// Drive runs one drive operation. A thrown error arrives as a JSON document
// inside a Go error string inside the run record; unwrapping it here is what
// lets the UI show "drive quota exceeded: ..." instead of "422".
func (c *bknClient) Drive(token string, input map[string]any) (json.RawMessage, error) {
	var res scriptResult
	err := c.post("/v1/script/drive/run", token, input, &res)
	if err != nil {
		var ae *apiError
		if errors.As(err, &ae) && ae.Status == http.StatusUnprocessableEntity {
			return nil, &apiError{Status: 422, Type: "drive_error", Message: ae.Message}
		}
		return nil, err
	}
	if res.Run.Status == "error" || !res.OK {
		return nil, &apiError{Status: 422, Type: "drive_error", Message: unwrapThrow(res.Run.Error)}
	}
	return res.Value, nil
}

// unwrapThrow digs the human sentence out of goja's error string, which looks
// like: Error: {"error":"..."} at fail (<eval>:35:9(15))
func unwrapThrow(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		var doc struct {
			Error string `json:"error"`
			Field string `json:"field"`
		}
		if err := json.Unmarshal([]byte(s[start:end+1]), &doc); err == nil && doc.Error != "" {
			return doc.Error
		}
	}
	if s == "" {
		return "the drive refused the operation"
	}
	return s
}

// Upload posts one file to the drive-upload hook.
func (c *bknClient) Upload(token string, body map[string]any) (json.RawMessage, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, c.Base+"/v1/hooks/drive-upload", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, &apiError{Status: 0, Type: "unreachable", Message: "bkn is unreachable: " + err.Error()}
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, errUnauthorized
	}
	if resp.StatusCode >= 400 {
		return nil, decodeErr(resp.StatusCode, out)
	}
	return out, nil
}

// SignedURL turns the path bkn signs into an absolute one the browser can
// follow -- against the PUBLIC base, not the one this process dials.
func (c *bknClient) SignedURL(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	base := c.Public
	if base == "" {
		base = c.Base
	}
	u, err := url.Parse(base + path)
	if err != nil {
		return base + path
	}
	return u.String()
}

// fetchURL is where THIS process reads a blob from: always the base it dials,
// never the public one, so a text preview does not take a trip through the
// public proxy to reach a server on the same machine.
func (c *bknClient) fetchURL(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return c.Base + path
}
