package tm

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// EnvBaseURL names the env var consulted for the default base URL
// when WithBaseURL isn't passed. Mirrors the CLI's convention so
// CI configurations can set the value once and use it from both
// the SDK and the `tm` binary.
const EnvBaseURL = "TM_BASE_URL"

// resolveDefaultBaseURL returns the base URL from $TM_BASE_URL,
// or an empty string if the env var isn't set. There's no
// built-in fallback — the SDK requires the caller to supply a
// base URL either via WithBaseURL or the env var. New() turns
// an empty result into a constructor error so misconfiguration
// surfaces at client-construction time, not on the first API
// call.
//
// (An earlier version returned a placeholder hosted URL
// `https://app.trafficmorph.example.com`, but that domain is in
// the IANA-reserved example.com TLD and doesn't resolve —
// "default that doesn't work" is worse than no default.)
func resolveDefaultBaseURL() string {
	v := strings.TrimSpace(os.Getenv(EnvBaseURL))
	if v == "" {
		return ""
	}
	return ensureTrailingSlash(v)
}

// validateBaseURL returns nil if raw parses as an absolute
// http/https URL with a non-empty host AND no query string or
// fragment, or a typed error otherwise. Catches the common
// malformed-input cases that would otherwise fail late on the
// first API call (or worse — silently misroute):
//
//   * `""` or whitespace-only        → "must not be empty"
//   * `"localhost:8080"` (no scheme) → "must include http:// or https:// scheme"
//   * `"ftp://x"` (wrong scheme)     → "scheme must be http or https, got "ftp""
//   * `"https://"` (no host)         → "must include a host"
//   * `"://garbage"` (unparseable)   → wraps url.Parse error
//   * `"https://x/?q=1"`             → "must not contain a query string"
//   * `"https://x/#frag"`            → "must not contain a fragment"
//
// Query strings and fragments are rejected because they corrupt
// the trailing-slash normalization (a naive append would produce
// "/?q=1/" — slash appended to the query, not the path) and
// silently mangle the generated request URLs. Callers who need
// per-request query params should add them at the endpoint call
// site, not bake them into the base URL.
//
// The string is trimmed before parsing; callers can pass
// "  https://x  \n" and have it normalize cleanly. Surrounding
// whitespace is almost always a copy-paste artifact rather than
// an intentional URL component.
func validateBaseURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return errors.New("base URL must not be empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("base URL %q is not a valid URL: %w", raw, err)
	}
	if u.Scheme == "" {
		return fmt.Errorf("base URL %q must include http:// or https:// scheme (got no scheme — did you mean http://%s?)", raw, raw)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("base URL %q has scheme %q; must be http or https", raw, u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("base URL %q must include a host", raw)
	}
	if u.RawQuery != "" {
		return fmt.Errorf("base URL %q must not contain a query string (have %q); attach per-request params at the endpoint call site instead", raw, u.RawQuery)
	}
	if u.Fragment != "" {
		return fmt.Errorf("base URL %q must not contain a fragment (have %q); fragments are client-side only and have no meaning to the server", raw, u.Fragment)
	}
	return nil
}

// normalizeBaseURL parses raw and returns it with a trailing
// slash on u.Path (NOT on the raw string). Call AFTER
// validateBaseURL so this isn't dragged through unparseable
// input — but be defensive even so: if the parse fails here
// somehow, fall back to the string-based append so we don't
// silently drop data.
//
// Why structural: the generated client resolves endpoint paths
// via `serverURL.Parse("./api/v1/...")`. Relative-URL resolution
// is path-aware — without a trailing slash on the path, the
// last segment is treated as a file and gets replaced. A
// previous string-based implementation appended "/" to the
// whole URL, which worked for path-only inputs but corrupted
// any URL with a query or fragment (the slash landed on the
// wrong component). validateBaseURL now rejects those inputs
// upstream, but doing the normalization on u.Path makes the
// function correct even if a future code path bypasses
// validation.
func normalizeBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		// Shouldn't happen — validateBaseURL would have caught
		// it. Defense-in-depth fallback to the historical string
		// behavior.
		return ensureTrailingSlash(raw)
	}
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}
	return u.String()
}

// ensureTrailingSlash is the legacy string-only trailing-slash
// helper. Used by resolveDefaultBaseURL for the env-var path
// (where validateBaseURL hasn't run yet — that check happens
// later in New()) and as the defensive fallback inside
// normalizeBaseURL. New code should prefer normalizeBaseURL,
// which is structural and handles query/fragment correctly.
func ensureTrailingSlash(url string) string {
	if strings.HasSuffix(url, "/") {
		return url
	}
	return url + "/"
}
