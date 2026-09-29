package api

import (
	"context"
	"net/http"
)

// The domains-list operation was generated as "List" before the spec gave
// it the explicit operationId "listDomains". These aliases keep tm-go v1
// callers compiling. Hand-written: oapi-codegen only owns client.gen.go.

// Deprecated: use ListDomainsResponse.
type ListResponse = ListDomainsResponse

// Deprecated: use NewListDomainsRequest.
func NewListRequest(server string) (*http.Request, error) {
	return NewListDomainsRequest(server)
}

// Deprecated: use ParseListDomainsResponse.
func ParseListResponse(rsp *http.Response) (*ListResponse, error) {
	return ParseListDomainsResponse(rsp)
}

// Deprecated: use ListDomains.
func (c *Client) List(ctx context.Context, reqEditors ...RequestEditorFn) (*http.Response, error) {
	return c.ListDomains(ctx, reqEditors...)
}

// Deprecated: use ListDomainsWithResponse.
func (c *ClientWithResponses) ListWithResponse(ctx context.Context, reqEditors ...RequestEditorFn) (*ListResponse, error) {
	return c.ListDomainsWithResponse(ctx, reqEditors...)
}
