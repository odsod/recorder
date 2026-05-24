// Package cdp provides a client for the Chrome DevTools Protocol.
//
// It supports two operations: listing debuggable browser tabs via the HTTP
// /json endpoint, and evaluating JavaScript expressions in a tab via the
// WebSocket-based Runtime.evaluate method. The WebSocket framing is implemented
// directly (no external dependencies) following RFC 6455.
package cdp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Tab represents a debuggable browser target returned by Chrome's /json endpoint.
type Tab struct {
	Title                string `json:"title"`
	URL                  string `json:"url"`
	Type                 string `json:"type"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

// ListTabsRequest specifies which Chrome instance to query.
type ListTabsRequest struct {
	// Port is the Chrome DevTools HTTP port (e.g. 9222).
	Port int
}

// ListTabsResponse contains the debuggable targets found.
type ListTabsResponse struct {
	Tabs []Tab
}

// EvaluateRequest specifies a JavaScript expression to execute in a browser tab.
type EvaluateRequest struct {
	// WebSocketURL is the tab's debugger WebSocket endpoint (from Tab.WebSocketDebuggerURL).
	WebSocketURL string
	// Expression is the JavaScript code to evaluate via Runtime.evaluate.
	Expression string
}

// EvaluateResponse contains the result of JavaScript evaluation.
type EvaluateResponse struct {
	// Value is the JSON-serialized return value of the expression.
	Value string
}

// Client communicates with Chrome via the DevTools Protocol.
type Client struct {
	http *http.Client
}

// New creates a Client. The HTTP client is used only for ListTabs;
// Evaluate opens a direct TCP/WebSocket connection.
func New(httpClient *http.Client) *Client {
	return &Client{http: httpClient}
}

// ListTabs fetches the list of debuggable targets from Chrome's /json endpoint.
func (c *Client) ListTabs(ctx context.Context, req ListTabsRequest) (ListTabsResponse, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	u := fmt.Sprintf("http://localhost:%d/json", req.Port)
	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodGet, u, nil)
	if err != nil {
		return ListTabsResponse{}, err
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return ListTabsResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ListTabsResponse{}, err
	}

	var tabs []Tab
	if err := json.Unmarshal(body, &tabs); err != nil {
		return ListTabsResponse{}, err
	}
	return ListTabsResponse{Tabs: tabs}, nil
}

type cdpRequest struct {
	ID     int       `json:"id"`
	Method string    `json:"method"`
	Params cdpParams `json:"params"`
}

type cdpParams struct {
	Expression    string `json:"expression"`
	ReturnByValue bool   `json:"returnByValue"`
}

type cdpResponse struct {
	ID     int `json:"id"`
	Result struct {
		Result struct {
			Value string `json:"value"`
		} `json:"result"`
	} `json:"result"`
}

// Evaluate executes a JavaScript expression in a browser tab via WebSocket.
// It opens a new WebSocket connection for each call, sends a Runtime.evaluate
// command, and returns the result value.
func (c *Client) Evaluate(ctx context.Context, req EvaluateRequest) (EvaluateResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := wsDial(ctx, req.WebSocketURL)
	if err != nil {
		return EvaluateResponse{}, fmt.Errorf("cdp dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	cdpReq := cdpRequest{
		ID:     1,
		Method: "Runtime.evaluate",
		Params: cdpParams{
			Expression:    req.Expression,
			ReturnByValue: true,
		},
	}

	payload, err := json.Marshal(cdpReq)
	if err != nil {
		return EvaluateResponse{}, fmt.Errorf("cdp marshal: %w", err)
	}

	if err := wsWrite(conn, payload); err != nil {
		return EvaluateResponse{}, fmt.Errorf("cdp write: %w", err)
	}

	respPayload, err := wsRead(conn)
	if err != nil {
		return EvaluateResponse{}, fmt.Errorf("cdp read: %w", err)
	}

	var cdpResp cdpResponse
	if err := json.Unmarshal(respPayload, &cdpResp); err != nil {
		return EvaluateResponse{}, fmt.Errorf("cdp unmarshal: %w", err)
	}

	return EvaluateResponse{Value: cdpResp.Result.Result.Value}, nil
}
