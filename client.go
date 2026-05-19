// Package tm is the Go SDK for the TrafficMorph v1 API.
//
// Quickstart:
//
//	import tm "github.com/trafficmorph-gif/tm-go"
//
//	c, err := tm.New("tm_…")  // reads TM_BASE_URL env if no opt given
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Typed access to every endpoint via the generated client at c.API.
//	resp, err := c.API.ListProfilesWithResponse(ctx)
//
// Authentication uses the `X-Api-Key` header (the alternative
// `Authorization: Bearer` form is also supported by the server).
// The client injects the header on every outbound request via a
// RequestEditor.
//
// Versioning: this SDK is generated from a versioned OpenAPI
// snapshot — see the `SpecVersion` constant for which v1 revision
// the binary was built against. The exported surface is the same
// shape produced by oapi-codegen 2.7.0; downstream code that
// imports the typed models (request / response structs) and
// endpoint methods will continue to compile across SDK minor
// versions as long as the server's `/api/v1` contract doesn't
// break.
package tm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/trafficmorph-gif/tm-go/api"
)

// SpecVersion records which `/api/v1` snapshot this SDK was
// generated against. Surfaced for callers that want to assert
// runtime API compatibility (e.g. log the SDK's spec revision
// alongside the server's `/v3/api-docs/v1` version). Bumped
// manually in the same PR that refreshes the snapshot.
const SpecVersion = "v1"

// DefaultUserAgent is sent on every request when the caller
// doesn't override via WithUserAgent. Includes both the SDK name
// AND the spec version so server-side observability can correlate
// client SDK revisions to API surface revisions.
const DefaultUserAgent = "tm-go-sdk/" + SpecVersion

// Default per-call timeout. Generous enough for the heaviest v1
// endpoint (`GET /history?…&size=100`) on a slow-network laptop;
// callers running latency-sensitive automation should override
// via WithTimeout.
const defaultTimeout = 30 * time.Second

// Client is the public SDK entry point. Wraps the generated typed
// client (exposed at Client.API for full endpoint access) with
// auth injection and a few ergonomic defaults.
//
// Construct with New; mutating fields after construction is not
// supported and races may corrupt in-flight requests.
type Client struct {
	// API exposes every generated endpoint method. Most callers
	// only need this — e.g. c.API.ListProfilesWithResponse(ctx).
	API *api.ClientWithResponses

	apiKey    string
	baseURL   string
	userAgent string
	timeout   time.Duration
	http      *http.Client
}

// Option mutates a Client's configuration during New. Pass to New
// after the apiKey:
//
//	c, err := tm.New("tm_…", tm.WithBaseURL("https://self-hosted.example.com"))
//
// Idiomatic Go functional-options pattern (kubectl, cobra, etc.).
type Option func(*Client) error

// WithBaseURL sets the API base URL. There is NO built-in default
// — calling New without this option falls back to $TM_BASE_URL,
// and if that env var is also unset, New returns a constructor
// error. Set this option (or the env var) to point at your
// TrafficMorph install: `http://localhost:8080` for local dev,
// `https://app.example.com` for a hosted deployment, etc.
//
// Malformed values (missing scheme, wrong scheme, no host) are
// rejected at construction time rather than failing late on the
// first API call with an opaque `unsupported protocol scheme`
// from net/http.
//
// Path-prefixed deployments work transparently. The generated
// client resolves endpoint paths via relative-URL parsing
// (`serverURL.Parse("./api/v1/...")`), which needs the base URL
// to end in `/` to preserve any path prefix — without the trailing
// slash, Go's net/url treats the last segment as a file and the
// prefix gets stripped during resolution. So a reverse-proxy
// mount like `https://internal.example.com/trafficmorph` produces
// the WRONG URL `https://internal.example.com/api/v1/profiles`
// instead of the intended `https://internal.example.com/trafficmorph/api/v1/profiles`.
// This option ENSURES the trailing slash (adds one if missing) so
// both spellings of the base URL behave the same way. An earlier
// revision did the opposite — TrimRight'd the slash off — which
// silently broke path-prefixed deployments.
func WithBaseURL(url string) Option {
	return func(c *Client) error {
		if err := validateBaseURL(url); err != nil {
			return fmt.Errorf("WithBaseURL: %w", err)
		}
		c.baseURL = normalizeBaseURL(url)
		return nil
	}
}

// WithHTTPClient injects a custom http.Client — useful for callers
// that need to thread their own transport (custom proxy, mTLS,
// telemetry middleware). The injected client's `Timeout` field is
// honored as the transport-level cap; the value from WithTimeout
// (or its default) is a SEPARATE per-call budget the caller wraps
// their context with — see WithTimeout for the why.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) error {
		if httpClient == nil {
			return errors.New("WithHTTPClient: httpClient must not be nil")
		}
		c.http = httpClient
		return nil
	}
}

// WithTimeout records a per-call timeout that callers retrieve
// via Client.Timeout() and apply themselves:
//
//	ctx, cancel := context.WithTimeout(ctx, c.Timeout())
//	defer cancel()
//	resp, err := c.API.ListProfilesWithResponse(ctx)
//
// The SDK deliberately does NOT auto-wrap call contexts. The
// generated endpoint methods take a context.Context per call —
// auto-wrapping at the RequestEditor layer is the wrong level
// (RequestEditors fire AFTER request construction, so cancellation
// would race against URL build / body marshalling). Surfacing the
// value as a config makes the policy visible and consistent across
// all callsites in an app, while keeping cancellation under the
// caller's control.
//
// Independently of this, http.Client.Timeout (set via
// WithHTTPClient, if provided) caps the transport-level duration
// — that's a hard ceiling for all calls regardless of the
// per-call context.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) error {
		if d <= 0 {
			return errors.New("WithTimeout: duration must be positive")
		}
		c.timeout = d
		return nil
	}
}

// WithUserAgent overrides the User-Agent header. Use to tag
// application-specific traffic in TrafficMorph's server logs (e.g.
// `my-app/1.2.3 (tm-go-sdk/v1)`). The SDK's default UA still
// identifies as `tm-go-sdk/<spec>` if this option isn't passed.
func WithUserAgent(ua string) Option {
	return func(c *Client) error {
		if ua == "" {
			return errors.New("WithUserAgent: ua must not be empty")
		}
		if err := validateHeaderValue(ua); err != nil {
			return fmt.Errorf("WithUserAgent: %w", err)
		}
		c.userAgent = ua
		return nil
	}
}

// validateHeaderValue rejects strings that would cause
// http.Request to fail at execution time with "invalid header
// field value". The Go stdlib's net/http transport runs the same
// check (via golang.org/x/net/http/httpguts.ValidHeaderFieldValue)
// when the header is actually set on a request — but that's deep
// inside the transport, far from where the user supplied the bad
// value. Catching it at SDK-construction time gives an error
// message the caller can act on.
//
// Per RFC 7230 §3.2.6, a header field-value is built from VCHAR
// (0x21-0x7E), SP (0x20), HTAB (0x09), and obs-text (0x80-0xFF).
// We reject anything outside that set: ASCII controls 0x00-0x1F
// other than HTAB, and DEL (0x7F). Mirroring the stdlib's rule
// exactly matters because a stricter check here than the
// transport's would let some bytes through that fail later
// anyway (the failure this validator exists to prevent).
//
// CR and LF in particular are called out by name in the error
// message because they enable header-injection attacks — an
// attacker supplying a key with embedded \r\n could splice in
// arbitrary extra headers — so the message wants to flag that
// case loudly even though the underlying rule is the broader
// "no controls".
func validateHeaderValue(v string) error {
	for i := 0; i < len(v); i++ {
		b := v[i]
		// Allowed: HTAB, SP through ~ (VCHAR + space), and the
		// obs-text range 0x80-0xFF.
		if b == '\t' || (b >= 0x20 && b != 0x7f) {
			continue
		}
		return fmt.Errorf("value contains %s at position %d; HTTP header values cannot contain ASCII control bytes (per RFC 7230, only HTAB, SP, VCHAR, and obs-text are allowed)", describeControlByte(b), i)
	}
	return nil
}

// describeControlByte names a rejected byte for inclusion in the
// validation error. The named cases (CR/LF/NUL/DEL) cover the
// values a caller is most likely to have stumbled into — a stray
// newline from a copy-paste, a NUL from a C-string boundary, a
// DEL from a corrupt env var. Anything else falls through to the
// hex form so the caller can grep the input for it.
func describeControlByte(b byte) string {
	switch b {
	case '\r':
		return "a carriage return (CR, 0x0D)"
	case '\n':
		return "a newline (LF, 0x0A)"
	case '\x00':
		return "a NUL byte (0x00)"
	case 0x7f:
		return "a DEL byte (0x7F)"
	default:
		return fmt.Sprintf("a control byte (0x%02X)", b)
	}
}

// New builds a Client. The apiKey must be the full `tm_…` value
// provisioned from the in-app Settings page; passing the empty
// string is rejected because it's almost certainly a forgotten
// env-var. The apiKey is also rejected if it contains any byte
// that net/http would refuse to send as a header value — ASCII
// controls 0x00-0x1F (except HTAB) and DEL (0x7F) — so a key
// pasted with a trailing newline or copied through a tool that
// inserted a stray control byte fails at construction time
// rather than mid-request with an opaque transport error. See
// validateHeaderValue for the full rule.
//
// A base URL is REQUIRED. Supply one via WithBaseURL or by
// setting $TM_BASE_URL before calling New. There is no built-in
// default — historical placeholder defaults were removed because
// they pointed at a reserved example.com host that doesn't
// resolve, producing the "default that doesn't work" anti-pattern.
//
// Returns an error if any option fails validation, the resolved
// base URL is empty / malformed, or the generated client can't
// be constructed. Construction never makes a network call.
func New(apiKey string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("apiKey must not be empty — provision one from the in-app Settings page")
	}
	if err := validateHeaderValue(apiKey); err != nil {
		return nil, fmt.Errorf("apiKey: %w", err)
	}
	c := &Client{
		apiKey:    apiKey,
		baseURL:   resolveDefaultBaseURL(),
		userAgent: DefaultUserAgent,
		timeout:   defaultTimeout,
		http:      &http.Client{},
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}
	// After options have run, the base URL must be set — either
	// from WithBaseURL or from $TM_BASE_URL. Empty here means
	// neither was supplied; fail with a message that names both
	// recovery paths so the caller knows what to do.
	if c.baseURL == "" {
		return nil, fmt.Errorf("base URL is required: pass tm.WithBaseURL(\"...\") or set $%s before calling tm.New", EnvBaseURL)
	}
	// If $TM_BASE_URL was set but malformed, resolveDefaultBaseURL
	// trusted it (and ran ensureTrailingSlash). Validate now so a
	// bad env-var value fails just as loudly as WithBaseURL would.
	if err := validateBaseURL(c.baseURL); err != nil {
		return nil, fmt.Errorf("$%s: %w", EnvBaseURL, err)
	}

	// Inject auth header on every outbound request via the
	// generated client's RequestEditor hook. Doing it once at
	// construction keeps the per-call API clean (callers never
	// touch headers).
	gen, err := api.NewClientWithResponses(c.baseURL,
		api.WithHTTPClient(c.http),
		api.WithRequestEditorFn(c.requestEditor()))
	if err != nil {
		return nil, fmt.Errorf("build generated client: %w", err)
	}
	c.API = gen
	return c, nil
}

// BaseURL returns the resolved base URL after option processing.
// Mostly useful for tests + diagnostic logs.
func (c *Client) BaseURL() string { return c.baseURL }

// Timeout returns the per-call timeout from WithTimeout (or the
// default). Callers should wrap their context with this value:
//
//	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout())
//	defer cancel()
//	resp, err := c.API.ListProfilesWithResponse(ctx)
func (c *Client) Timeout() time.Duration { return c.timeout }

// requestEditor injects X-Api-Key + User-Agent on every request.
// X-Api-Key (not Authorization: Bearer) for the same reason the
// CLI uses it: easier to disambiguate from JWT / OAuth flows in
// access-log inspection.
func (c *Client) requestEditor() api.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		req.Header.Set("X-Api-Key", c.apiKey)
		req.Header.Set("User-Agent", c.userAgent)
		return nil
	}
}
