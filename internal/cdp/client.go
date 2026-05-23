package cdp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/odsod/recorder/internal/httpclient"
)

type Tab struct {
	URL                  string `json:"url"`
	Type                 string `json:"type"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

func ListTabs(ctx context.Context, port int) ([]Tab, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("http://localhost:%d/json", port)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := httpclient.Shared.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tabs []Tab
	if err := json.Unmarshal(body, &tabs); err != nil {
		return nil, err
	}
	return tabs, nil
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

func Eval(ctx context.Context, wsURL string, js string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		return "", fmt.Errorf("cdp dial: %w", err)
	}
	defer conn.CloseNow()

	req := cdpRequest{
		ID:     1,
		Method: "Runtime.evaluate",
		Params: cdpParams{
			Expression:    js,
			ReturnByValue: true,
		},
	}

	if err := wsjson.Write(ctx, conn, req); err != nil {
		return "", fmt.Errorf("cdp write: %w", err)
	}

	var resp cdpResponse
	if err := wsjson.Read(ctx, conn, &resp); err != nil {
		return "", fmt.Errorf("cdp read: %w", err)
	}

	conn.Close(websocket.StatusNormalClosure, "")
	return resp.Result.Result.Value, nil
}
