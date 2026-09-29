package api

import (
	"context"
	"net/http"
	"testing"
)

// Compile-time pins for the pre-listDomains call shapes; see compat.go.
var (
	_ func(context.Context, ...RequestEditorFn) (*http.Response, error) = (&Client{}).List
	_ func(context.Context, ...RequestEditorFn) (*ListResponse, error)  = (&ClientWithResponses{}).ListWithResponse
	_ func(string) (*http.Request, error)                               = NewListRequest
	_ func(*http.Response) (*ListResponse, error)                       = ParseListResponse
)

func TestListRequestTargetsDomains(t *testing.T) {
	req, err := NewListRequest("https://tm.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got := req.URL.Path; got != "/api/v1/domains" {
		t.Fatalf("NewListRequest path = %q, want /api/v1/domains", got)
	}
}
