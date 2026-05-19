# tm-go — TrafficMorph Go SDK

```
████████╗██████╗  █████╗ ███████╗███████╗██╗ ██████╗
╚══██╔══╝██╔══██╗██╔══██╗██╔════╝██╔════╝██║██╔════╝
   ██║   ██████╔╝███████║█████╗  █████╗  ██║██║
   ██║   ██╔══██╗██╔══██║██╔══╝  ██╔══╝  ██║██║
   ██║   ██║  ██║██║  ██║██║     ██║     ██║╚██████╗
   ╚═╝   ╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝     ╚═╝ ╚═════╝
        ███╗   ███╗ ██████╗ ██████╗ ██████╗ ██╗  ██╗
        ████╗ ████║██╔═══██╗██╔══██╗██╔══██╗██║  ██║
        ██╔████╔██║██║   ██║██████╔╝██████╔╝███████║
        ██║╚██╔╝██║██║   ██║██╔══██╗██╔═══╝ ██╔══██║
        ██║ ╚═╝ ██║╚██████╔╝██║  ██║██║     ██║  ██║
        ╚═╝     ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚═╝     ╚═╝  ╚═╝
```

[![Go Reference](https://pkg.go.dev/badge/github.com/trafficmorph-gif/tm-go.svg)](https://pkg.go.dev/github.com/trafficmorph-gif/tm-go)
[![Latest tag](https://img.shields.io/github/v/tag/trafficmorph-gif/tm-go?sort=semver&label=latest)](https://github.com/trafficmorph-gif/tm-go/tags)
[![Go Report Card](https://goreportcard.com/badge/github.com/trafficmorph-gif/tm-go)](https://goreportcard.com/report/github.com/trafficmorph-gif/tm-go)
[![Go version](https://img.shields.io/github/go-mod/go-version/trafficmorph-gif/tm-go)](go.mod)
[![License](https://img.shields.io/github/license/trafficmorph-gif/tm-go)](LICENSE)

Typed Go client for the TrafficMorph `/api/v1` surface. Generated
from the project's OpenAPI snapshot, so request and response
shapes are statically typed at compile time.

API reference docs live on [pkg.go.dev](https://pkg.go.dev/github.com/trafficmorph-gif/tm-go).

## Install

```bash
# Quick start — always picks up the latest tagged release.
go get github.com/trafficmorph-gif/tm-go@latest

# Reproducible builds (CI / production) — pin to an exact tag.
go get github.com/trafficmorph-gif/tm-go@v0.1.7
```

## Prerequisites

- **Go 1.25 or newer** — declared minimum in [`go.mod`](go.mod).
- **A TrafficMorph API key** in the form `tm_…`. Provision one from the in-app **Settings → API keys** page.
- **A reachable TrafficMorph install.** The examples below assume `http://localhost:8080` (a server running locally via `mvnw spring-boot:run`); swap for your hosted URL otherwise. There is no built-in default — the SDK requires the base URL to be set explicitly.

## Quickstart

Export the two required values before running the program, so the snippet works as a single copy-paste:

```bash
export TM_API_KEY="tm_your_key_here"
export TM_BASE_URL="http://localhost:8080"   # or your hosted TrafficMorph URL
```

Then:

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
        tm.WithBaseURL(os.Getenv("TM_BASE_URL")),
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

### First successful call checklist

- [ ] `tm_…` API key exported as `TM_API_KEY` (or hard-coded in the call to `tm.New`).
- [ ] `TM_BASE_URL` points at a reachable TrafficMorph server (`http://localhost:8080` for local dev).
- [ ] Program prints `status: 200 OK, N bytes` — an empty profile list is `[]`, so N is typically ≥ 2.
- [ ] `resp.Body` holds the JSON payload. Decode it per [Decoding responses](#decoding-responses) below.

If the program errored before reaching the first line, jump to [Common errors](#common-errors).

## Decoding responses

`resp.Body` is the raw HTTP body as `[]byte`. The generated `*WithResponse` types intentionally don't carry typed `JSON200` fields yet (the upstream OpenAPI spec doesn't declare typed response schemas; tracked as a follow-up). Decode with `encoding/json` and the typed structs from the `api` sub-package — replace the `log.Printf("status: …")` line in the Quickstart with:

```go
switch resp.StatusCode() {
case 200:
    var profiles []api.TrafficProfileSummaryResponse
    if err := json.Unmarshal(resp.Body, &profiles); err != nil {
        log.Fatalf("decode: %v", err)
    }
    for _, p := range profiles {
        if p.Id != nil && p.Name != nil {
            fmt.Printf("profile %d: %s\n", *p.Id, *p.Name)
        }
    }

case 400, 401, 403, 404:
    var apiErr struct {
        Error string `json:"error"`
    }
    _ = json.Unmarshal(resp.Body, &apiErr)
    log.Fatalf("server returned %s: %s", resp.Status(), apiErr.Error)

default:
    log.Fatalf("unexpected status %d: %s", resp.StatusCode(), resp.Body)
}
```

Add `"encoding/json"`, `"fmt"`, and `"github.com/trafficmorph-gif/tm-go/api"` to the import block.

Pointer fields in the typed structs (`*int64`, `*string`, ...) reflect OpenAPI optional-field semantics — always nil-check before dereferencing.

## Next steps

Three common flows after `ListProfiles`:

### Create a profile

```go
body := api.CreateProfileJSONRequestBody{
    // Populate per api.ApiProfileRequest — field shapes live on pkg.go.dev:
    // https://pkg.go.dev/github.com/trafficmorph-gif/tm-go/api#ApiProfileRequest
}
resp, err := c.API.CreateProfileWithResponse(ctx, body)
```

### Start a run

```go
// profileID comes from ListProfiles or CreateProfile.
resp, err := c.API.StartWithResponse(ctx, profileID)
// resp.StatusCode() == 200 → run started.
// Poll GetProfile (or the runs endpoints) for status transitions.
```

### View recent history

```go
size := int32(20)
resp, err := c.API.ListHistoryWithResponse(ctx, &api.ListHistoryParams{
    Size: &size,
    // Other optional filters: ProfileId, TriggeredBy, Region, AutoVerdict, Tag.
})
```

For the full method list, see [What's in the box](#whats-in-the-box). Method signatures and request/response struct shapes are documented on [pkg.go.dev](https://pkg.go.dev/github.com/trafficmorph-gif/tm-go).

## Common errors

All errors below come from `tm.New(...)` — they surface at construction time, before any network call, so you don't need to set up the rest of your app to hit them.

| Error fragment | Cause | Fix |
|---|---|---|
| `apiKey must not be empty` | First arg to `tm.New` is `""` — usually a missing `TM_API_KEY` env var. | Pass the literal key or `export TM_API_KEY=...` before running. |
| `apiKey: value contains a carriage return …` (or newline, NUL, DEL, other control byte) | API key has stray whitespace / control chars (a common copy-paste artifact). | Only the literal `tm_…` characters belong in the value; strip surrounding whitespace. |
| `base URL is required: pass tm.WithBaseURL(...) or set $TM_BASE_URL` | No `WithBaseURL` option AND no `$TM_BASE_URL` env var. | Pass `tm.WithBaseURL("http://…")` or export `TM_BASE_URL`. |
| `WithBaseURL: base URL "…" must include http:// or https:// scheme` | Base URL is missing the protocol (e.g. `localhost:8080`). | Add the scheme: `http://localhost:8080`. |
| `WithBaseURL: base URL "…" has scheme "…"; must be http or https` | Non-`http`/`https` scheme (e.g. `ftp://…`). | Use `http://` or `https://`. |
| `WithBaseURL: base URL "…" must not contain a query string` | Base URL has `?key=value` appended. | Strip the query — attach per-request params at the endpoint call site instead. |
| `WithBaseURL: base URL "…" must not contain a fragment` | Base URL has `#foo` appended. | Strip the fragment — fragments are client-side only and meaningless to the server. |
| `$TM_BASE_URL: …` (any of the above) | Env-supplied base URL fails the same checks. | Same fixes; the prefix names the source so you know whether the option or the env var was at fault. |

## Configuration

| Source | Precedence |
|--------|------------|
| Function options (`WithBaseURL`, `WithTimeout`, ...) | Highest |
| Environment variables (`TM_BASE_URL`) | Middle |
| Built-in defaults | Lowest |

| Option | Env var | Default | Notes |
|---|---|---|---|
| `WithBaseURL` | `TM_BASE_URL` | none — required | Points at your TrafficMorph install. See [Base URL rules](#base-url-rules) for accepted/rejected shapes. |
| `WithTimeout` | — | `30s` | Per-call timeout. The SDK does NOT auto-apply this; callers wrap their context: `ctx, cancel := context.WithTimeout(ctx, c.Timeout())`. |
| `WithHTTPClient` | — | `&http.Client{}` | Inject a custom transport (proxies, mTLS, telemetry middleware). |
| `WithUserAgent` | — | `tm-go-sdk/<spec-version>` | Override to tag app traffic in server logs (e.g. `my-app/1.2.3 (tm-go-sdk/v1)`). |

### Base URL rules

Option and env values flow through the same validate-and-normalize pipeline, so they produce identical results for the same logical input. The option wins on conflict.

**Accepted shapes** — any absolute `http://` or `https://` URL with a non-empty host:

- `http://localhost:8080` — typical local dev.
- `https://app.example.com` — hosted deployment.
- `https://host/proxy-prefix` — reverse-proxy mount; the prefix is preserved during URL resolution.
- `https://host/a%2Fb` — percent-encoded path segments stay verbatim. Per RFC 3986, `/a%2Fb` (one segment, containing a literal slash) and `/a/b` (two segments) are semantically different paths — the SDK never collapses one into the other.

The SDK appends a trailing slash if missing, so both spellings (with or without) produce the same final value.

**Rejected at construction time** — clear error from `tm.New`, not a late transport failure:

| Bad input | Error fragment |
|---|---|
| `""` or whitespace-only | `must not be empty` |
| `localhost:8080` (no scheme) | `must include … scheme` |
| `ftp://x` (wrong scheme) | `must be http or https` |
| `https://` (no host) | `must include a host` |
| `https://x/?q=1` (query) | `must not contain a query string` |
| `https://x/#frag` (fragment) | `must not contain a fragment` |

Query strings and fragments are refused because they belong on per-request URLs, not the deployment root.

## Authentication

The SDK sends every request with `X-Api-Key: tm_…`. The alternative `Authorization: Bearer tm_…` form documented for the public API works too — both map to the same backing filter on the server side — but the SDK uses `X-Api-Key` because it disambiguates from JWT / OAuth in server access logs.

## What's in the box

```
github.com/trafficmorph-gif/tm-go     ← public package (Client, New, options)
github.com/trafficmorph-gif/tm-go/api ← generated typed client + request/response structs (reachable via Client.API)
```

Most callers only need the top-level package. The generated `api` sub-package is exposed so callers can reach the typed request/response structs (e.g. `api.TrafficProfileSummaryResponse`) and the typed endpoint methods (e.g. `c.API.ListProfilesWithResponse`).

Endpoint coverage matches the server's `/api/v1` surface 1:1:

| Tag | Methods |
|---|---|
| Profiles | `ListProfiles`, `CreateProfile`, `GetProfile`, `UpdateProfile`, `DeleteProfile` |
| Runs | `Start`, `Stop`, `Pause`, `Resume` |
| History | `ListHistory`, `GetHistoryItem` |
| Domains | `ListDomains`, `Add`, `VerifyDns`, `VerifyHttp`, `Remove` |
| Captures | `Analyse`, `ImportCapture` |
| Variable sets | `ListVariablesSets`, `Create`, `Get`, `Rename`, `ChangeMode`, `Delete` |

Each endpoint exposes both a raw `*http.Response` form and a typed `*WithResponse` form. Prefer the latter for status-code + body access in one struct — see [Decoding responses](#decoding-responses) for the pattern.

## Versioning

| | Symbol | Meaning |
|---|---|---|
| SDK release | `vMAJOR.MINOR.PATCH` git tag | Bumped per SDK cut; pin via `go get github.com/trafficmorph-gif/tm-go@vX.Y.Z` |
| Spec snapshot | `tm.SpecVersion` constant (currently `"v1"`) | Tracks which `/api/v1` revision the SDK was generated against |

A given SDK release is built against exactly one OpenAPI snapshot. Server-side `/api/v1` changes always preserve backwards compatibility within the v1 line — a `v0.3.0` SDK works against the same server as a `v0.1.0` SDK, as long as both target `/api/v1`.
