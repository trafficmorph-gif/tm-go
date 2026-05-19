package tm_test

import (
	"context"
	"fmt"
	"log"
	"time"

	tm "github.com/trafficmorph-gif/tm-go"
)

// Example_listProfiles renders on pkg.go.dev as the SDK's
// quickstart sample. Covers the most common flow: construct the
// client, hit a typed endpoint, decode the typed response.
func Example_listProfiles() {
	c, err := tm.New("tm_replace_me",
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
	if resp.StatusCode() != 200 {
		log.Fatalf("server returned %s: %s", resp.Status(), resp.Body)
	}
	// resp.Body carries the raw bytes; oapi-codegen would expose
	// resp.JSON200 here if the OpenAPI spec carried a response
	// schema — see the README for decoding patterns.
	fmt.Printf("got %d bytes of profile list\n", len(resp.Body))
}

// Example_startRunAndWait shows the CI-gate shape: kick off a run
// and poll until it has a verdict. The full polling helper is in
// the CLI; library users typically write their own gate logic on
// top of the typed endpoints.
func Example_startRunAndWait() {
	// Always check the constructor error. A missing/malformed base
	// URL, a key containing control bytes, or any failing Option
	// returns (nil, err) — c.API on a nil client would panic.
	// Copy-paste-friendly examples must model this correctly even
	// though it adds one line.
	//
	// The base URL is REQUIRED: pass tm.WithBaseURL("...") as
	// shown here, or export $TM_BASE_URL before calling tm.New
	// (the SDK reads the env var when the option isn't passed).
	c, err := tm.New("tm_replace_me",
		tm.WithBaseURL("https://app.trafficmorph.example.com"))
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Phase 1: start. Handle the transport-level error BEFORE
	// touching `resp` — on a network failure the generated client
	// returns `(nil, err)`, so reading resp.Status() inside the
	// same branch panics. Two checks instead of one keeps copy-
	// paste users safe.
	resp, err := c.API.StartWithResponse(ctx, 42 /* profileId */)
	if err != nil {
		log.Fatalf("start failed: %v", err)
	}
	if resp.StatusCode() != 200 {
		log.Fatalf("start returned %s: %s", resp.Status(), resp.Body)
	}

	// Phase 2: poll the profile detail for status transition.
	// (Production code would use exponential backoff + a verdict
	// timeout — see the `tm` CLI's runs.go for a reference
	// implementation.)
	for {
		profileResp, err := c.API.GetProfileWithResponse(ctx, 42)
		if err != nil {
			log.Fatal(err)
		}
		_ = profileResp // inspect status, break when terminal
		time.Sleep(5 * time.Second)
		break // (loop trimmed for the docs example)
	}
}
