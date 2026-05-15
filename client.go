// Package tm is the Go SDK for the TrafficMorph v1 API.
//
// Quickstart:
//
//	import tm "github.com/trafficmorph/tm-go"
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

	"github.com/trafficmorph/tm-go/api"
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

// WithBaseURL overrides the API base URL. Default is taken from
// $TM_BASE_URL or, if unset, the public hosted instance. Set this
// for self-hosted deployments or for pointing at a staging
// environment.
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
		if url == "" {
			return errors.New("WithBaseURL: url must not be empty")
		}
		c.baseURL = ensureTrailingSlash(url)
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
		c.userAgent = ua
		return nil
	}
}

// New builds a Client. The apiKey must be the full `tm_…` value
// provisioned from the in-app Settings page; passing the empty
// string is rejected because it's almost certainly a forgotten
// env-var.
//
// Returns an error if any option fails validation or the
// generated client can't be constructed against the resolved base
// URL. Construction never makes a network call.
func New(apiKey string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("apiKey must not be empty — provision one from the in-app Settings page")
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
