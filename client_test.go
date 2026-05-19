package tm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestNew_RejectsEmptyAPIKey locks in the "no silent
// auth-omission" contract — passing the empty string is almost
// always a forgotten env-var, and a Client built without auth
// would fail later inside the first API call with a confusing
// 401. Failing fast at New time gives the caller a clear
// "you forgot the key" signal.
func TestNew_RejectsEmptyAPIKey(t *testing.T) {
	_, err := New("")
	if err == nil {
		t.Fatal("expected error for empty apiKey")
	}
	if !strings.Contains(err.Error(), "apiKey") {
		t.Errorf("error should mention apiKey; got: %v", err)
	}
}

// TestNew_ResolvesBaseURLFromEnv pins the env-var precedence
// chain. The CLI sets the convention; the SDK matches it so a CI
// step exporting TM_BASE_URL once gets used by both. The
// trailing-slash normalization applies to env-sourced URLs too —
// otherwise a CI env-var spelling without a slash would silently
// drop the path prefix.
func TestNew_ResolvesBaseURLFromEnv(t *testing.T) {
	t.Setenv(EnvBaseURL, "https://staging.example.com")
	c, err := New("tm_test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Trailing slash added by ensureTrailingSlash — see baseurl.go.
	if c.BaseURL() != "https://staging.example.com/" {
		t.Errorf("BaseURL should come from env (with normalized slash); got %q", c.BaseURL())
	}
}

// TestNew_WithBaseURL_OverridesEnv — explicit option wins over
// env (standard CLI / SDK config precedence).
func TestNew_WithBaseURL_OverridesEnv(t *testing.T) {
	t.Setenv(EnvBaseURL, "https://env-says.example.com")
	c, err := New("tm_test", WithBaseURL("https://flag-says.example.com"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.BaseURL() != "https://flag-says.example.com/" {
		t.Errorf("explicit option should override env; got %q", c.BaseURL())
	}
}

// TestNew_WithBaseURL_NormalizesTrailingSlash — the generated
// client resolves endpoint paths via relative-URL parsing, which
// REQUIRES a trailing slash on the base URL to preserve any path
// prefix. Both spellings of the base URL must normalize to the
// same trailing-slash form so a missing slash from a CI env var
// doesn't silently drop the path prefix.
func TestNew_WithBaseURL_NormalizesTrailingSlash(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://example.com", "https://example.com/"},
		{"https://example.com/", "https://example.com/"},
		{"https://example.com/prefix", "https://example.com/prefix/"},
		{"https://example.com/prefix/", "https://example.com/prefix/"},
		// Nested prefixes — common in reverse-proxy deployments.
		{"https://host/team/app", "https://host/team/app/"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			cli, err := New("tm_test", WithBaseURL(c.in))
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if cli.BaseURL() != c.want {
				t.Errorf("BaseURL: got %q, want %q", cli.BaseURL(), c.want)
			}
		})
	}
}

// TestNew_WithBaseURL_PreservesPathPrefix is the wire-level
// regression guard for the path-prefixed-deployment bug. Earlier
// revisions TrimRight'd the trailing slash off the base URL,
// which (combined with the generated client's
// `serverURL.Parse("./api/v1/...")` style of relative resolution)
// stripped the path prefix during URL resolution. A reverse-proxy
// mount like `https://host/proxy-prefix` would silently hit
// `https://host/api/v1/...` instead.
//
// httptest gives us a real server to verify the path that
// actually reaches the wire — pure unit tests on BaseURL() can't
// catch this class of bug because the resolution happens inside
// the generated client.
func TestNew_WithBaseURL_PreservesPathPrefix(t *testing.T) {
	var seenPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	cases := []struct {
		name string
		base string
	}{
		{"prefix with trailing slash", srv.URL + "/proxy-prefix/"},
		{"prefix without trailing slash", srv.URL + "/proxy-prefix"},
		// Multi-segment prefix — common in nested reverse-proxy
		// configs like `team-name/app-name`.
		{"multi-segment prefix without trailing slash", srv.URL + "/team/app"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			seenPath = ""
			cli, err := New("tm_test", WithBaseURL(c.base))
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if _, err := cli.API.ListProfilesWithResponse(context.Background()); err != nil {
				t.Fatalf("ListProfiles: %v", err)
			}
			// Pull the path-prefix portion (everything BEFORE
			// /api/v1) out of the wire-level path; we want to
			// assert it survived the relative-URL resolution.
			idx := strings.Index(seenPath, "/api/v1")
			if idx < 0 {
				t.Fatalf("server saw %q, expected ../api/v1/...", seenPath)
			}
			gotPrefix := seenPath[:idx]
			// The path prefix from the original base URL must
			// still be there. Strip leading slash before comparison
			// so both `/proxy-prefix` and `/team/app` cases work.
			wantPrefix := strings.TrimSuffix(c.base[len(srv.URL):], "/")
			if gotPrefix != wantPrefix {
				t.Errorf("path prefix dropped: server saw %q, want prefix %q (full path %q)",
					gotPrefix, wantPrefix, seenPath)
			}
		})
	}
}

// TestOptions_ValidationErrors locks each option's input
// validation. These are caller-side mistakes (empty URL, nil
// http.Client, zero timeout) that would produce subtle failures
// later — fail fast at construction time.
func TestOptions_ValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		opt  Option
		want string
	}{
		{"empty base URL", WithBaseURL(""), "base URL must not be empty"},
		{"nil http client", WithHTTPClient(nil), "httpClient must not be nil"},
		{"zero timeout", WithTimeout(0), "duration must be positive"},
		{"negative timeout", WithTimeout(-1 * time.Second), "duration must be positive"},
		{"empty user agent", WithUserAgent(""), "ua must not be empty"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := New("tm_test", c.opt)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q should mention %q", err.Error(), c.want)
			}
		})
	}
}

// TestRequestEditor_InjectsAuthAndUserAgent — end-to-end smoke
// test: spin up an httptest server, point the SDK at it, make a
// real API call, verify the headers landed on the server side.
// This is the only true regression guard that proves the
// RequestEditor wiring works — pure unit tests can't observe what
// hits the wire.
func TestRequestEditor_InjectsAuthAndUserAgent(t *testing.T) {
	var seenAPIKey, seenUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAPIKey = r.Header.Get("X-Api-Key")
		seenUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		// Minimal empty-list response that satisfies ListProfiles.
		// Empty array decodes cleanly into []TrafficProfileSummaryResponse
		// in the generated client.
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c, err := New("tm_secret_xyz",
		WithBaseURL(srv.URL),
		WithUserAgent("my-app/1.2.3 (tm-go-sdk/v1)"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := c.API.ListProfilesWithResponse(ctx)
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	if resp.StatusCode() != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode())
	}
	if seenAPIKey != "tm_secret_xyz" {
		t.Errorf("X-Api-Key not injected; got %q", seenAPIKey)
	}
	if seenUA != "my-app/1.2.3 (tm-go-sdk/v1)" {
		t.Errorf("User-Agent not injected; got %q", seenUA)
	}
}

// TestDefaultUserAgent confirms the SDK identifies itself when the
// caller doesn't override. Observability tooling on the server
// keys off this to attribute traffic to the SDK vs. CLI vs. raw
// curl.
func TestDefaultUserAgent(t *testing.T) {
	var seenUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c, _ := New("tm_test", WithBaseURL(srv.URL))
	_, _ = c.API.ListProfilesWithResponse(context.Background())
	if !strings.HasPrefix(seenUA, "tm-go-sdk/") {
		t.Errorf("default User-Agent should start with tm-go-sdk/; got %q", seenUA)
	}
	if !strings.Contains(seenUA, SpecVersion) {
		t.Errorf("default User-Agent should include SpecVersion %q; got %q", SpecVersion, seenUA)
	}
}

// TestNew_WithHTTPClient — confirms a custom http.Client is
// actually plumbed through and used. The mechanism: route the
// SDK through a transport whose RoundTrip records the call, so we
// can assert the SDK's request reached our custom transport.
type recordingTransport struct {
	called bool
	inner  http.RoundTripper
}

func (rt *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.called = true
	return rt.inner.RoundTrip(req)
}

func TestNew_WithHTTPClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	rt := &recordingTransport{inner: http.DefaultTransport}
	custom := &http.Client{Transport: rt}

	c, err := New("tm_test", WithBaseURL(srv.URL), WithHTTPClient(custom))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, _ = c.API.ListProfilesWithResponse(context.Background())
	if !rt.called {
		t.Error("custom http.Client.Transport should have been called")
	}
}

// TestWithBaseURL_RejectsMalformedURLs locks in fail-fast for the
// caller-side mistakes that used to fail late at request time with
// `unsupported protocol scheme "localhost"` or similar opaque
// errors from net/http. Catching at constructor time names exactly
// what's wrong with the input.
func TestWithBaseURL_RejectsMalformedURLs(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string // substring the error must contain
	}{
		{"missing scheme", "localhost:8080", "scheme"},
		{"wrong scheme", "ftp://example.com", "http or https"},
		{"no host", "https://", "host"},
		{"whitespace only", "   ", "must not be empty"},
		{"empty", "", "must not be empty"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := New("tm_test", WithBaseURL(c.in))
			if err == nil {
				t.Fatalf("expected error for %q", c.in)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q should mention %q", err.Error(), c.want)
			}
		})
	}
}

// TestWithBaseURL_AcceptsWhitespacePadding — surrounding whitespace
// is almost always a copy-paste artifact (env var with trailing
// newline, etc.). The SDK trims it during normalization rather than
// rejecting outright.
func TestWithBaseURL_AcceptsWhitespacePadding(t *testing.T) {
	c, err := New("tm_test", WithBaseURL("  https://example.com  "))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.BaseURL() != "https://example.com/" {
		t.Errorf("BaseURL should be trimmed + slash-normalized; got %q", c.BaseURL())
	}
}

// TestNew_RejectsInvalidHeaderChars guards against header injection
// at the apiKey boundary. Without validation, a caller passing a
// key containing `\r\nX-Injected: yes` would have it silently fail
// inside net/http with "invalid header field value" — far from
// the New() callsite. Reject at construction time with a message
// that names the bad byte.
func TestNew_RejectsInvalidHeaderChars(t *testing.T) {
	cases := []struct {
		name string
		key  string
		want string
	}{
		{"carriage return", "tm_x\rmore", "carriage return"},
		{"newline", "tm_x\nInjected: yes", "newline"},
		{"NUL byte", "tm_x\x00", "NUL byte"},
		{"CRLF injection attempt", "tm_x\r\nX-Injected: yes", "carriage return"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := New(c.key, WithBaseURL("https://example.com"))
			if err == nil {
				t.Fatalf("expected error for key %q", c.key)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q should mention %q", err.Error(), c.want)
			}
			if !strings.Contains(err.Error(), "apiKey") {
				t.Errorf("error %q should mention apiKey", err.Error())
			}
		})
	}
}

// TestWithUserAgent_RejectsInvalidHeaderChars — same fail-fast
// contract as for apiKey, but for the User-Agent header. CR/LF
// here would similarly fail late inside net/http transport.
func TestWithUserAgent_RejectsInvalidHeaderChars(t *testing.T) {
	cases := []struct {
		name string
		ua   string
		want string
	}{
		{"carriage return", "my-app\rfoo", "carriage return"},
		{"newline", "my-app\nfoo", "newline"},
		{"NUL byte", "my-app\x00", "NUL byte"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := New("tm_test",
				WithBaseURL("https://example.com"),
				WithUserAgent(c.ua))
			if err == nil {
				t.Fatalf("expected error for UA %q", c.ua)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q should mention %q", err.Error(), c.want)
			}
			if !strings.Contains(err.Error(), "WithUserAgent") {
				t.Errorf("error %q should mention WithUserAgent", err.Error())
			}
		})
	}
}

// TestNew_RequiresBaseURL — the SDK no longer ships a built-in
// default base URL (the previous placeholder was in the reserved
// example.com TLD and didn't resolve). With no WithBaseURL option
// AND no $TM_BASE_URL env var, New() must error at construction
// time naming both recovery paths.
func TestNew_RequiresBaseURL(t *testing.T) {
	t.Setenv(EnvBaseURL, "") // ensure env unset
	_, err := New("tm_test")
	if err == nil {
		t.Fatal("expected error when no base URL configured")
	}
	if !strings.Contains(err.Error(), "WithBaseURL") {
		t.Errorf("error should mention WithBaseURL option; got: %v", err)
	}
	if !strings.Contains(err.Error(), EnvBaseURL) {
		t.Errorf("error should mention %s env var; got: %v", EnvBaseURL, err)
	}
}

// TestNew_RejectsMalformedTMBaseURLEnv — a bad env-var value
// shouldn't fail late at request time the same way a bad
// WithBaseURL argument used to. Validate env input at constructor
// time with a message that names the env var so the caller knows
// where to look.
func TestNew_RejectsMalformedTMBaseURLEnv(t *testing.T) {
	t.Setenv(EnvBaseURL, "localhost:8080") // missing scheme
	_, err := New("tm_test")
	if err == nil {
		t.Fatal("expected error for malformed TM_BASE_URL env")
	}
	if !strings.Contains(err.Error(), EnvBaseURL) {
		t.Errorf("error should mention %s env var; got: %v", EnvBaseURL, err)
	}
	if !strings.Contains(err.Error(), "scheme") {
		t.Errorf("error should mention scheme; got: %v", err)
	}
}
