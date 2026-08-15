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

// resolveDefaultBaseURL returns the trimmed value of $TM_BASE_URL,
// or an empty string if the env var isn't set. There's no
// built-in fallback — the SDK requires the caller to supply a
// base URL either via WithBaseURL or the env var. New() turns
// an empty result into a constructor error so misconfiguration
// surfaces at client-construction time, not on the first API
// call.
//
// Trim-only on purpose: validation and normalization happen
// downstream in New() so env-sourced values go through the SAME
// pipeline as WithBaseURL inputs. Two reasons:
//
//   1. Parity. Without unified normalization, the option and env
//      paths can produce DIFFERENT BaseURL() strings for the same
//      logical input — e.g. an input containing `%2F` (encoded
//      slash) survives string-based normalization but gets
//      decoded into a literal `/` by structural normalization.
//      Routing depends on which spelling the caller used, which
//      is the worst kind of bug to debug.
//
//   2. Cleaner diagnostics. Pre-appending `/` here meant validate
//      saw `https://x?q=1/` for an env value of `https://x?q=1`,
//      and the resulting error reported a query string of `q=1/`
//      that the user never typed. Returning raw keeps the error
//      message faithful to what the user actually set.
//
// (An earlier version returned a placeholder hosted URL
// `https://app.trafficmorph.example.com`, but that domain is in
// the IANA-reserved example.com TLD and doesn't resolve —
// "default that doesn't work" is worse than no default.)
func resolveDefaultBaseURL() string {
	return strings.TrimSpace(os.Getenv(EnvBaseURL))
}

// validateBaseURL returns nil if raw parses as an absolute
// http/https URL with a non-empty host AND no query string or
// fragment, or a typed error otherwise. Catches the common
// malformed-input cases that would otherwise fail late on the
// first API call (or worse — silently misroute):
//
//   * `""` or whitespace-only        → "must not be empty"
//   * `"localhost:8092"` (no scheme) → "must include http:// or https:// scheme"
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
	// Reject both `?foo=bar` (RawQuery populated) AND a bare
	// trailing `?` (RawQuery empty but u.ForceQuery is true, which
	// makes url.URL.String() emit the `?` anyway). The bare-`?`
	// case isn't a meaningful query string but still corrupts
	// trailing-slash expectations: the emitted form ends with `?`,
	// not `/`, and the generated client's relative-URL resolution
	// then drops path segments.
	if u.RawQuery != "" || u.ForceQuery {
		have := u.RawQuery
		if have == "" {
			have = "(empty, but trailing `?` is present)"
		}
		return fmt.Errorf("base URL %q must not contain a query string (have %q); attach per-request params at the endpoint call site instead", raw, have)
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
//
// RawPath preservation: url.Parse populates u.RawPath only when
// the input's path encoding differs from the default re-encoding
// of u.Path — typically when the path contains percent-encoded
// reserved characters like `%2F` (encoded slash). If we appended
// only to u.Path, url.URL.String() would re-escape from Path using
// default rules and turn `%2F` into a literal `/` — silently
// rewriting routing. Appending the same trailing slash to BOTH
// keeps RawPath a valid encoding of Path, so String() prefers
// RawPath and the original encoding survives end-to-end. Without
// this, WithBaseURL and TM_BASE_URL would produce different
// BaseURL() strings for the same logical input.
func normalizeBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		// Shouldn't happen — validateBaseURL would have caught
		// it. Defense-in-depth fallback to the historical string
		// behavior.
		return ensureTrailingSlash(raw)
	}

	// Whether the URL already ends with a trailing slash in its
	// emitted form is NOT always answered by HasSuffix(u.Path, "/")
	// — for inputs like `http://x/%2F`, url.Parse stores
	// `u.Path = "//"` (the `%2F` decodes to `/`) while
	// `u.RawPath = "/%2F"`. u.String() emits RawPath, so the
	// emitted form ends with `F`, NOT `/`. Skipping the append
	// based on the decoded suffix here would silently produce a
	// no-trailing-slash base URL, reintroducing the
	// path-prefix-dropped bug. Inspect RawPath when it's set.
	var hasTrailingSlash bool
	if u.RawPath != "" {
		hasTrailingSlash = strings.HasSuffix(u.RawPath, "/")
	} else {
		hasTrailingSlash = strings.HasSuffix(u.Path, "/")
	}

	if !hasTrailingSlash {
		u.Path += "/"
		if u.RawPath != "" {
			// RawPath is set when the input had non-default
			// encoding (e.g. `%2F`). Tack the slash onto the
			// encoded form too so url.URL.String() emits the
			// preserved encoding rather than re-escaping Path.
			u.RawPath += "/"
		}
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
