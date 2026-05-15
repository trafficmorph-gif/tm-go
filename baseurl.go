package tm

import (
	"os"
	"strings"
)

// EnvBaseURL names the env var consulted for the default base URL
// when WithBaseURL isn't passed. Mirrors the CLI's convention so
// CI configurations can set the value once and use it from both
// the SDK and the `tm` binary.
const EnvBaseURL = "TM_BASE_URL"

// defaultPublicHostedURL is the fallback when neither WithBaseURL
// nor $TM_BASE_URL is set. Points at the SaaS instance; on-prem
// callers will always need to override via env or option.
const defaultPublicHostedURL = "https://app.trafficmorph.example.com"

// resolveDefaultBaseURL walks the precedence chain
//
//	env $TM_BASE_URL  →  defaultPublicHostedURL
//
// and normalizes the result via ensureTrailingSlash. Extracted to
// its own file so tests can stub the env-var path without
// reaching across the client.go file's larger surface.
func resolveDefaultBaseURL() string {
	v := os.Getenv(EnvBaseURL)
	if v == "" {
		v = defaultPublicHostedURL
	}
	return ensureTrailingSlash(v)
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
