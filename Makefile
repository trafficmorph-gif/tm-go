# Go SDK build / regen / test targets.
#
#   make test           run all tests
#   make regen-client   regenerate api/client.gen.go from the
#                       canonical OpenAPI snapshot at ../cli/openapi/v1.json
#   make tidy           go mod tidy
#
# The SDK doesn't ship a binary, so there's no `make build`.
# `go test ./...` exercises the public surface and the generated
# code's compile correctness in one pass.

OAPI_CODEGEN_VERSION = v2.7.0
# Single source of truth for the API snapshot is the CLI's
# openapi/v1.json — keeping ONE snapshot avoids the drift window
# where the CLI gets regenerated and the SDK doesn't (or vice
# versa).
SPEC = ../cli/openapi/v1.json

.PHONY: test tidy regen-client

test:
	go test ./...

tidy:
	go mod tidy

# regen-client uses `go run` with a pinned oapi-codegen version
# instead of installing a binary into ./bin/. Same rationale as
# the CLI's Makefile: avoids checking in platform-specific
# binaries, and the Go module cache handles per-platform
# compilation transparently.
regen-client:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) \
		-config openapi/codegen.yaml $(SPEC)
