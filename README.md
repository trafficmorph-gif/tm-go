# tm-go — TrafficMorph Go SDK

Typed Go client for the TrafficMorph `/api/v1` surface. Generated
from the same versioned OpenAPI snapshot as the [`tm` CLI](../cli/)
so the two stay in lockstep.

```
go get github.com/trafficmorph-gif/tm-go@v0.1.0
```

## Quickstart

```go
package main

import (
    "context"
    "log"
    "time"

    tm "github.com/trafficmorph-gif/tm-go"
)

func main() {
    c, err := tm.New("tm_…",
        tm.WithBaseURL("https://app.trafficmorph.example.com"),
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

The SDK injects an `X-Api-Key: tm_…` header on every request. The
alternative `Authorization: Bearer tm_…` form documented for the
public API works too — both schemes map to the same backing
filter on the server side — but the SDK uses `X-Api-Key` because
it disambiguates from JWT / OAuth in access logs.

Provision the key from the in-app **Settings → API keys** page.

On the cloud build, API access is a TEAM+ feature. Self-hosted
installs (`app.deployment-mode=SELF_HOSTED`) give every
authenticated user full access regardless of stored plan tier.

## Configuration

| Source | Precedence |
|--------|------------|
| Function options (`WithBaseURL`, `WithTimeout`, ...) | Highest |
| Environment variables (`TM_BASE_URL`) | Middle |
| Built-in defaults | Lowest |

| Option | Env var | Default | Notes |
|---|---|---|---|
| `WithBaseURL` | `TM_BASE_URL` | `https://app.trafficmorph.example.com` | Override per environment. Either spelling (with or without trailing slash) works — the SDK normalizes to a trailing-slash form so path-prefixed deployments behind reverse proxies (`https://host/proxy-prefix`) keep their prefix during URL resolution. |
| `WithTimeout` | — | `30s` | Per-call timeout. The SDK does NOT auto-apply this; callers wrap their context with `context.WithTimeout(ctx, c.Timeout())`. |
| `WithHTTPClient` | — | `&http.Client{}` | Inject a custom transport for proxies, mTLS, telemetry. |
| `WithUserAgent` | — | `tm-go-sdk/v1` | Override to tag app traffic in server logs (e.g. `my-app/1.2.3 (tm-go-sdk/v1)`). |

## What's in the box

```
github.com/trafficmorph-gif/tm-go     ← the public package (Client, New, options)
github.com/trafficmorph-gif/tm-go/api ← generated typed client + DTOs (re-export via Client.API)
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
| Domains | `List` (domains), `Add`, `VerifyDns`, `VerifyHttp`, `Remove` |
| Captures | `Analyse`, `ImportCapture` |

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
| SDK release | `vMAJOR.MINOR.PATCH` git tag | Bumped per SDK cut; tracked in `go.mod` consumers via `go get` |
| Spec snapshot | `tm.SpecVersion` constant (currently `"v1"`) | Tracks which `/api/v1` revision the SDK was generated against. Bump alongside `make -C cli regen-spec` |

A given SDK release is built against exactly one OpenAPI snapshot.
Server-side `/api/v1` changes always preserve backwards
compatibility within the v1 line — a v0.3.0 SDK works against the
same server as a v0.1.0 SDK, as long as both target `/api/v1`.

## Regenerating after a server-side API change

```
make -C cli regen-spec      # writes ../cli/openapi/v1.json (and .yaml)
make -C sdk-go regen-client # regenerates this module's api/client.gen.go
make -C sdk-go test         # smoke-check the new client
```

Then commit both the snapshot and the regenerated `api/client.gen.go`
in the same PR — the SDK is built against the committed snapshot,
not against a server response at build time.

## See also

- [`tm` CLI](../cli/) — single-binary wrapper that also imports
  this SDK's generated types. Designed for CI gating: `tm runs
  start <id> --wait --fail-on-verdict FAIL`.
- [OpenAPI spec](../cli/openapi/v1.json) — the canonical
  `/api/v1` snapshot both the CLI and SDK build from.
- Server-side docs at `/swagger-ui` on any TrafficMorph
  deployment.
