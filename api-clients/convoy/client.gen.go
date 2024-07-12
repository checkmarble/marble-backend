// Package convoy provides primitives to interact with the openapi HTTP API.
//
// Code generated by github.com/oapi-codegen/oapi-codegen/v2 version v2.3.0 DO NOT EDIT.
package convoy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/oapi-codegen/runtime"
)

const (
	ApiKeyAuthScopes = "ApiKeyAuth.Scopes"
)

// HandlersStub defines model for handlers.Stub.
type HandlersStub = map[string]interface{}

// ModelsFanoutEvent defines model for models.FanoutEvent.
type ModelsFanoutEvent struct {
	// CustomHeaders Specifies custom headers you want convoy to add when the event is dispatched to your endpoint
	CustomHeaders *map[string]string `json:"custom_headers,omitempty"`

	// Data Data is an arbitrary JSON value that gets sent as the body of the
	// webhook to the endpoints
	Data *map[string]interface{} `json:"data,omitempty"`

	// EventType Event Type is used for filtering and debugging e.g invoice.paid
	EventType *string `json:"event_type,omitempty"`

	// IdempotencyKey Specify a key for event deduplication
	IdempotencyKey *string `json:"idempotency_key,omitempty"`

	// OwnerId Used for fanout, sends this event to all endpoints with this OwnerID.
	OwnerId *string `json:"owner_id,omitempty"`
}

// UtilServerResponse defines model for util.ServerResponse.
type UtilServerResponse struct {
	Message *string `json:"message,omitempty"`
	Status  *bool   `json:"status,omitempty"`
}

// CreateEndpointFanoutEventJSONRequestBody defines body for CreateEndpointFanoutEvent for application/json ContentType.
type CreateEndpointFanoutEventJSONRequestBody = ModelsFanoutEvent

// RequestEditorFn  is the function signature for the RequestEditor callback function
type RequestEditorFn func(ctx context.Context, req *http.Request) error

// Doer performs HTTP requests.
//
// The standard http.Client implements this interface.
type HttpRequestDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client which conforms to the OpenAPI3 specification for this service.
type Client struct {
	// The endpoint of the server conforming to this interface, with scheme,
	// https://api.deepmap.com for example. This can contain a path relative
	// to the server, such as https://api.deepmap.com/dev-test, and all the
	// paths in the swagger spec will be appended to the server.
	Server string

	// Doer for performing requests, typically a *http.Client with any
	// customized settings, such as certificate chains.
	Client HttpRequestDoer

	// A list of callbacks for modifying requests which are generated before sending over
	// the network.
	RequestEditors []RequestEditorFn
}

// ClientOption allows setting custom parameters during construction
type ClientOption func(*Client) error

// Creates a new Client, with reasonable defaults
func NewClient(server string, opts ...ClientOption) (*Client, error) {
	// create a client with sane default values
	client := Client{
		Server: server,
	}
	// mutate client and add all optional params
	for _, o := range opts {
		if err := o(&client); err != nil {
			return nil, err
		}
	}
	// ensure the server URL always has a trailing slash
	if !strings.HasSuffix(client.Server, "/") {
		client.Server += "/"
	}
	// create httpClient, if not already present
	if client.Client == nil {
		client.Client = &http.Client{}
	}
	return &client, nil
}

// WithHTTPClient allows overriding the default Doer, which is
// automatically created using http.Client. This is useful for tests.
func WithHTTPClient(doer HttpRequestDoer) ClientOption {
	return func(c *Client) error {
		c.Client = doer
		return nil
	}
}

// WithRequestEditorFn allows setting up a callback function, which will be
// called right before sending the request. This can be used to mutate the request.
func WithRequestEditorFn(fn RequestEditorFn) ClientOption {
	return func(c *Client) error {
		c.RequestEditors = append(c.RequestEditors, fn)
		return nil
	}
}

// The interface specification for the client above.
type ClientInterface interface {
	// CreateEndpointFanoutEventWithBody request with any body
	CreateEndpointFanoutEventWithBody(ctx context.Context, projectID string, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*http.Response, error)

	CreateEndpointFanoutEvent(ctx context.Context, projectID string, body CreateEndpointFanoutEventJSONRequestBody, reqEditors ...RequestEditorFn) (*http.Response, error)
}

func (c *Client) CreateEndpointFanoutEventWithBody(ctx context.Context, projectID string, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewCreateEndpointFanoutEventRequestWithBody(c.Server, projectID, contentType, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

func (c *Client) CreateEndpointFanoutEvent(ctx context.Context, projectID string, body CreateEndpointFanoutEventJSONRequestBody, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewCreateEndpointFanoutEventRequest(c.Server, projectID, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// NewCreateEndpointFanoutEventRequest calls the generic CreateEndpointFanoutEvent builder with application/json body
func NewCreateEndpointFanoutEventRequest(server string, projectID string, body CreateEndpointFanoutEventJSONRequestBody) (*http.Request, error) {
	var bodyReader io.Reader
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	bodyReader = bytes.NewReader(buf)
	return NewCreateEndpointFanoutEventRequestWithBody(server, projectID, "application/json", bodyReader)
}

// NewCreateEndpointFanoutEventRequestWithBody generates requests for CreateEndpointFanoutEvent with any type of body
func NewCreateEndpointFanoutEventRequestWithBody(server string, projectID string, contentType string, body io.Reader) (*http.Request, error) {
	var err error

	var pathParam0 string

	pathParam0, err = runtime.StyleParamWithLocation("simple", false, "projectID", runtime.ParamLocationPath, projectID)
	if err != nil {
		return nil, err
	}

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/v1/projects/%s/events/fanout", pathParam0)
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", queryURL.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", contentType)

	return req, nil
}

func (c *Client) applyEditors(ctx context.Context, req *http.Request, additionalEditors []RequestEditorFn) error {
	for _, r := range c.RequestEditors {
		if err := r(ctx, req); err != nil {
			return err
		}
	}
	for _, r := range additionalEditors {
		if err := r(ctx, req); err != nil {
			return err
		}
	}
	return nil
}

// ClientWithResponses builds on ClientInterface to offer response payloads
type ClientWithResponses struct {
	ClientInterface
}

// NewClientWithResponses creates a new ClientWithResponses, which wraps
// Client with return type handling
func NewClientWithResponses(server string, opts ...ClientOption) (*ClientWithResponses, error) {
	client, err := NewClient(server, opts...)
	if err != nil {
		return nil, err
	}
	return &ClientWithResponses{client}, nil
}

// WithBaseURL overrides the baseURL.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) error {
		newBaseURL, err := url.Parse(baseURL)
		if err != nil {
			return err
		}
		c.Server = newBaseURL.String()
		return nil
	}
}

// ClientWithResponsesInterface is the interface specification for the client with responses above.
type ClientWithResponsesInterface interface {
	// CreateEndpointFanoutEventWithBodyWithResponse request with any body
	CreateEndpointFanoutEventWithBodyWithResponse(ctx context.Context, projectID string, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*CreateEndpointFanoutEventResponse, error)

	CreateEndpointFanoutEventWithResponse(ctx context.Context, projectID string, body CreateEndpointFanoutEventJSONRequestBody, reqEditors ...RequestEditorFn) (*CreateEndpointFanoutEventResponse, error)
}

type CreateEndpointFanoutEventResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	JSON200      *struct {
		Data    *HandlersStub `json:"data,omitempty"`
		Message *string       `json:"message,omitempty"`
		Status  *bool         `json:"status,omitempty"`
	}
	JSON400 *struct {
		Data    *HandlersStub `json:"data,omitempty"`
		Message *string       `json:"message,omitempty"`
		Status  *bool         `json:"status,omitempty"`
	}
	JSON401 *struct {
		Data    *HandlersStub `json:"data,omitempty"`
		Message *string       `json:"message,omitempty"`
		Status  *bool         `json:"status,omitempty"`
	}
	JSON404 *struct {
		Data    *HandlersStub `json:"data,omitempty"`
		Message *string       `json:"message,omitempty"`
		Status  *bool         `json:"status,omitempty"`
	}
}

// Status returns HTTPResponse.Status
func (r CreateEndpointFanoutEventResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r CreateEndpointFanoutEventResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

// CreateEndpointFanoutEventWithBodyWithResponse request with arbitrary body returning *CreateEndpointFanoutEventResponse
func (c *ClientWithResponses) CreateEndpointFanoutEventWithBodyWithResponse(ctx context.Context, projectID string, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*CreateEndpointFanoutEventResponse, error) {
	rsp, err := c.CreateEndpointFanoutEventWithBody(ctx, projectID, contentType, body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseCreateEndpointFanoutEventResponse(rsp)
}

func (c *ClientWithResponses) CreateEndpointFanoutEventWithResponse(ctx context.Context, projectID string, body CreateEndpointFanoutEventJSONRequestBody, reqEditors ...RequestEditorFn) (*CreateEndpointFanoutEventResponse, error) {
	rsp, err := c.CreateEndpointFanoutEvent(ctx, projectID, body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseCreateEndpointFanoutEventResponse(rsp)
}

// ParseCreateEndpointFanoutEventResponse parses an HTTP response from a CreateEndpointFanoutEventWithResponse call
func ParseCreateEndpointFanoutEventResponse(rsp *http.Response) (*CreateEndpointFanoutEventResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &CreateEndpointFanoutEventResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 200:
		var dest struct {
			Data    *HandlersStub `json:"data,omitempty"`
			Message *string       `json:"message,omitempty"`
			Status  *bool         `json:"status,omitempty"`
		}
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON200 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 400:
		var dest struct {
			Data    *HandlersStub `json:"data,omitempty"`
			Message *string       `json:"message,omitempty"`
			Status  *bool         `json:"status,omitempty"`
		}
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON400 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 401:
		var dest struct {
			Data    *HandlersStub `json:"data,omitempty"`
			Message *string       `json:"message,omitempty"`
			Status  *bool         `json:"status,omitempty"`
		}
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON401 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 404:
		var dest struct {
			Data    *HandlersStub `json:"data,omitempty"`
			Message *string       `json:"message,omitempty"`
			Status  *bool         `json:"status,omitempty"`
		}
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON404 = &dest

	}

	return response, nil
}