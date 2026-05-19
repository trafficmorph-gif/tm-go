package tm

import (
	"strings"
	"testing"
)

// FuzzBaseURLValidationAndNormalization pins three invariants
// over the validate/normalize pair against arbitrary inputs:
//
//  1. Neither function panics. The URL parser is robust, but a
//     future change might introduce an out-of-bounds slice or
//     similar in our additional logic; we want a regression
//     guard that catches it before users do.
//
//  2. normalizeBaseURL is idempotent on inputs that validate.
//     A double normalization (e.g. WithBaseURL ran, code reads
//     c.baseURL and re-normalizes downstream) must produce the
//     same string. The unified pipeline in New() relies on this
//     — the option path effectively normalizes twice (once via
//     WithBaseURL's now-defunct internal pass, kept idempotent
//     for defense-in-depth, then once at the end of New()).
//
//  3. validateBaseURL accepts normalizeBaseURL's output. A
//     caller pattern of "validate raw, store norm(raw), later
//     re-validate stored" must not produce intermittent failures.
//
//  4. Validated, normalized URLs end with `/` on the path. This
//     is the precondition the generated client's relative-URL
//     resolution depends on — if it ever stops holding, the
//     path-prefixed-deployment bug returns and is silently
//     destructive.
//
// We don't fuzz the option/env parity invariant here because
// asserting it requires mutating $TM_BASE_URL, which is
// process-global and interacts badly with fuzz workers running
// in parallel. TestNew_OptionAndEnv_PathEncodingParity covers
// that invariant for representative encoded-path inputs.
//
// Run with:  go test -run=^$ -fuzz=FuzzBaseURLValidationAndNormalization
// CI just runs the seed corpus (via plain `go test`), which is
// cheap and gives us a smoke test on every push.
func FuzzBaseURLValidationAndNormalization(f *testing.F) {
	seeds := []string{
		// Valid representative cases — exercise happy paths.
		"https://example.com",
		"https://example.com/",
		"https://example.com/team/app",
		"https://example.com/team/app/",
		"http://localhost:8080",
		"http://localhost:8080/",
		// Percent-encoded path: validate accepts, normalize must
		// preserve the encoding so `%2F` doesn't collapse to `/`.
		"https://example.com/a%2Fb",
		"https://example.com/foo%21bar",
		// Already-trailing-slash on encoded path.
		"https://example.com/a%2Fb/",
		// Whitespace tolerance.
		"  https://example.com  ",
		// Known-rejected cases — exercise validate's error paths.
		"",
		"   ",
		"localhost:8080",
		"ftp://example.com",
		"https://",
		"https://example.com/?x=1",
		"https://example.com/#frag",
		// Garbage / odd shapes.
		"://garbage",
		"https://x\x00.example.com",
		"   \t\n",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		// Invariant 1a: validate doesn't panic, even on garbage.
		validateErr := safeValidate(t, raw)

		// Invariant 1b: normalize doesn't panic either — including
		// on inputs that validate rejects. The defense-in-depth
		// fallback inside normalize must be panic-safe too.
		norm := safeNormalize(t, raw)

		if validateErr != nil {
			// Nothing more to assert. validate's error wording for
			// different shapes of bad input isn't a fuzzed property.
			return
		}

		// Invariant 2: idempotence.
		norm2 := safeNormalize(t, norm)
		if norm != norm2 {
			t.Errorf("normalize not idempotent: %q → %q → %q", raw, norm, norm2)
		}

		// Invariant 3: validate accepts normalize's output.
		if err := validateBaseURL(norm); err != nil {
			t.Errorf("validate rejected normalize output: raw %q → norm %q → err %v",
				raw, norm, err)
		}

		// Invariant 4: trailing slash on path. Because validate
		// already rejected query/fragment by this point, norm has
		// neither, and a simple HasSuffix check on the whole string
		// is equivalent to checking the path portion.
		if !strings.HasSuffix(norm, "/") {
			t.Errorf("normalize output %q has no trailing slash (path-prefix bug would return)", norm)
		}
	})
}

// safeValidate calls validateBaseURL and converts a panic into a
// t.Fatalf so the fuzz failure points at the input and the
// panic message. testing.F's default behavior on panic is fine
// for go test, but trapping here gives a cleaner failure record.
func safeValidate(t *testing.T, raw string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("validateBaseURL(%q) panicked: %v", raw, r)
		}
	}()
	return validateBaseURL(raw)
}

// safeNormalize is the panic-trapping wrapper for normalizeBaseURL.
func safeNormalize(t *testing.T, raw string) (out string) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("normalizeBaseURL(%q) panicked: %v", raw, r)
		}
	}()
	return normalizeBaseURL(raw)
}
