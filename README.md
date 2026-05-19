# tm-go — TrafficMorph Go SDK

Typed Go client for the TrafficMorph `/api/v1` surface. Generated
from the project's OpenAPI snapshot, so request and response
shapes are statically typed at compile time.

```
go get github.com/trafficmorph-gif/tm-go@v0.1.2
```

API reference docs live on [pkg.go.dev](https://pkg.go.dev/github.com/trafficmorph-gif/tm-go).

## Quickstart

```go
package main

import (
    "context"
    "log"
    "os"
    "time"

    tm "github.com/trafficmorph-gif/tm-go"
)

func main() {
    c, err := tm.New(os.Getenv("TM_API_KEY"),
        tm.WithBaseURL(os.Getenv("TM_BASE_URL")), // e.g. http://localhost:8080 or your TrafficMorph host
        tm.WithTimeout(15*time.Second))
    if err != nil {
        log.Fatal(err)
    }

    ctx, cancel := context.WithTimeout(context.Background(), c.Timeout())
    defer cancel()

    resp, err := c.API.ListProfilesWithResponse(ctx)
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("status: %s, %d bytes", resp.Status(), len(resp.Body))
}
```

## Authentication

The SDK sends every request with `X-Api-Key: tm_…`. The
alternative `Authorization: Bearer tm_…` form documented for the
public API works too — both map to the same backing filter on
the server side — but the SDK uses `X-Api-Key` because it
disambiguates from JWT / OAuth in server access logs.

Provision the key from the in-app **Settings → API keys** page.

On the cloud build, API access is a TEAM+ feature. Self-hosted
installs (`app.deployment-mode=SELF_HOSTED`) give every
authenticated user full API access regardless of stored plan
tier.

## Configuration

| Source | Precedence |
|--------|------------|
| Function options (`WithBaseURL`, `WithTimeout`, ...) | Highest |
| Environment variables (`TM_BASE_URL`) | Middle |
| Built-in defaults | Lowest |

| Option | Env var | Default | Notes |
|---|---|---|---|
| `WithBaseURL` | `TM_BASE_URL` | none — must be set explicitly | Point at your TrafficMorph install (`http://localhost:8080` for local dev, or your hosted URL). Calling `tm.New()` without either `WithBaseURL(...)` or `$TM_BASE_URL` is a constructor error. Either spelling (with or without trailing slash) works — the SDK normalizes to a trailing-slash form so path-prefixed deployments behind reverse proxies (`https://host/proxy-prefix`) keep their prefix during URL resolution. Malformed values (missing scheme, wrong scheme, no host) are rejected at construction time rather than failing late at request time. |
| `WithTimeout` | — | `30s` | Per-call timeout. The SDK does NOT auto-apply this; callers wrap their context with `context.WithTimeout(ctx, c.Timeout())`. |
| `WithHTTPClient` | — | `&http.Client{}` | Inject a custom transport for proxies, mTLS, telemetry. |
| `WithUserAgent` | — | `tm-go-sdk/v1` | Override to tag app traffic in server logs (e.g. `my-app/1.2.3 (tm-go-sdk/v1)`). |

## What's in the box

```
github.com/trafficmorph-gif/tm-go     ← public package (Client, New, options)
github.com/trafficmorph-gif/tm-go/api ← generated typed client + DTOs (reachable via Client.API)
```

Most callers only need the top-level package. The generated `api`
sub-package is exposed so callers can reach the typed request /
response DTOs (e.g. `api.TrafficProfileSummaryResponse`) and the
typed endpoint methods (e.g. `c.API.ListProfilesWithResponse`).

Endpoint coverage matches the server's `/api/v1` surface 1:1:

| Tag | Methods |
|---|---|
| Profiles | `ListProfiles`, `CreateProfile`, `GetProfile`, `UpdateProfile`, `DeleteProfile` |
| Runs | `Start`, `Stop`, `Pause`, `Resume` |
| History | `ListHistory`, `GetHistoryItem` |
| Domains | `ListDomains`, `Add`, `VerifyDns`, `VerifyHttp`, `Remove` |
| Captures | `Analyse`, `ImportCapture` |
| Variables sets | `ListVariablesSets`, `Create`, `Get`, `Rename`, `ChangeMode`, `Delete` |

Each endpoint exposes both a raw `*http.Response` form and a typed
`*WithResponse` form. Prefer the latter for status-code + body
access in one struct:

```go
resp, err := c.API.GetProfileWithResponse(ctx, profileID)
if err != nil { ... }
switch resp.StatusCode() {
case 200:
    // resp.Body has the JSON; decode into api.ApiProfileResponse if needed
case 400:
    // resp.Body has {"error":"..."} per the OpenAPI contract
}
```

## Versioning

| | Symbol | Meaning |
|---|---|---|
| SDK release | `vMAJOR.MINOR.PATCH` git tag | Bumped per SDK cut; pin via `go get github.com/trafficmorph-gif/tm-go@vX.Y.Z` |
| Spec snapshot | `tm.SpecVersion` constant (currently `"v1"`) | Tracks which `/api/v1` revision the SDK was generated against |

A given SDK release is built against exactly one OpenAPI snapshot.
Server-side `/api/v1` changes always preserve backwards
compatibility within the v1 line — a `v0.3.0` SDK works against
the same server as a `v0.1.0` SDK, as long as both target
`/api/v1`.
