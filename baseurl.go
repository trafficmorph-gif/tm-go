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
// http/https URL with a non-empty host, or a typed error
// otherwise. Catches the common malformed-input cases that would
// otherwise fail late on the first API call:
//
//   * `""` or whitespace-only        → "must not be empty"
//   * `"localhost:8080"` (no scheme) → "must include http:// or https:// scheme"
//   * `"ftp://x"` (wrong scheme)     → "scheme must be http or https, got "ftp""
//   * `"https://"` (no host)         → "must include a host"
//   * `"://garbage"` (unparseable)   → wraps url.Parse error
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
	return nil
}

// normalizeBaseURL trims surrounding whitespace and ensures a
// trailing slash. Call AFTER validateBaseURL so this isn't
// dragged through unparseable input. Extracted so WithBaseURL
// and resolveDefaultBaseURL can share normalization without
// duplicating the trim.
func normalizeBaseURL(raw string) string {
	return ensureTrailingSlash(strings.TrimSpace(raw))
}

// ensureTrailingSlash normalizes a base URL for use with the
// generated client's relative-URL resolution. See the WithBaseURL
// godoc for the why: without a trailing slash, path-prefixed
// deployments lose their prefix during URL.Parse(relative). Both
// the WithBaseURL option AND the env / default fallback must
// apply this normalization, otherwise self-hosted callers who set
// $TM_BASE_URL with no slash would hit the same bug.
func ensureTrailingSlash(url string) string {
	if strings.HasSuffix(url, "/") {
		return url
	}
	return url + "/"
}
