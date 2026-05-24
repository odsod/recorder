package cdp_test

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/odsod/recorder/internal/protocol/cdp"
)

func TestListTabs_Success(t *testing.T) {
	tabs := []cdp.Tab{
		{
			Title:                "Google",
			URL:                  "https://google.com",
			Type:                 "page",
			WebSocketDebuggerURL: "ws://localhost:9222/devtools/page/1",
		},
		{Title: "Extension", URL: "chrome-extension://abc", Type: "other", WebSocketDebuggerURL: ""},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json" {
			t.Errorf("expected /json, got %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(tabs)
	}))
	defer srv.Close()

	port, _ := strconv.Atoi(strings.Split(srv.Listener.Addr().String(), ":")[1])

	client := cdp.New(srv.Client())
	resp, err := client.ListTabs(context.Background(), cdp.ListTabsRequest{Port: port})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Tabs) != 2 {
		t.Fatalf("expected 2 tabs, got %d", len(resp.Tabs))
	}
	if resp.Tabs[0].Title != "Google" {
		t.Errorf("expected 'Google', got %q", resp.Tabs[0].Title)
	}
	if resp.Tabs[0].WebSocketDebuggerURL != "ws://localhost:9222/devtools/page/1" {
		t.Errorf("unexpected ws URL: %s", resp.Tabs[0].WebSocketDebuggerURL)
	}
}

func TestListTabs_ConnectionRefused(t *testing.T) {
	client := cdp.New(&http.Client{Timeout: time.Second})
	_, err := client.ListTabs(context.Background(), cdp.ListTabsRequest{Port: 1})
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestEvaluate_Success(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()

	port := ln.Addr().(*net.TCPAddr).Port
	wsURL := fmt.Sprintf("ws://127.0.0.1:%d/devtools/page/1", port)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		buf := make([]byte, 4096)
		n, _ := conn.Read(buf)
		request := string(buf[:n])
		if !strings.Contains(request, "Upgrade: websocket") {
			t.Errorf("missing upgrade header")
			return
		}

		response := "HTTP/1.1 101 Switching Protocols\r\n" +
			"Upgrade: websocket\r\n" +
			"Connection: Upgrade\r\n" +
			"Sec-WebSocket-Accept: dummy\r\n" +
			"\r\n"
		_, _ = conn.Write([]byte(response))

		frame := readWSFrame(t, conn)

		var cdpReq struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
			Params struct {
				Expression    string `json:"expression"`
				ReturnByValue bool   `json:"returnByValue"`
			} `json:"params"`
		}
		if err := json.Unmarshal(frame, &cdpReq); err != nil {
			t.Errorf("unmarshal cdp request: %v", err)
			return
		}
		if cdpReq.Method != "Runtime.evaluate" {
			t.Errorf("expected Runtime.evaluate, got %s", cdpReq.Method)
		}
		if cdpReq.Params.Expression != "1+1" {
			t.Errorf("expected expression '1+1', got %q", cdpReq.Params.Expression)
		}

		cdpResp := map[string]any{
			"id": cdpReq.ID,
			"result": map[string]any{
				"result": map[string]any{
					"value": strings.Repeat("x", 200),
				},
			},
		}
		respData, _ := json.Marshal(cdpResp)
		writeWSFrame(conn, respData)
	}()

	client := cdp.New(&http.Client{})
	resp, err := client.Evaluate(context.Background(), cdp.EvaluateRequest{
		WebSocketURL: wsURL, Expression: "1+1",
	})
	if err != nil {
		t.Fatal(err)
	}
	expected := strings.Repeat("x", 200)
	if resp.Value != expected {
		t.Errorf("expected 200 x's, got %d chars", len(resp.Value))
	}
}

func TestEvaluate_ConnectionRefused(t *testing.T) {
	client := cdp.New(&http.Client{})
	_, err := client.Evaluate(context.Background(), cdp.EvaluateRequest{
		WebSocketURL: "ws://127.0.0.1:1/devtools/page/1", Expression: "1",
	})
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestEvaluate_NonUpgradeResponse(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()

	port := ln.Addr().(*net.TCPAddr).Port
	wsURL := fmt.Sprintf("ws://127.0.0.1:%d/devtools/page/1", port)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
		_, _ = conn.Write([]byte("HTTP/1.1 403 Forbidden\r\nContent-Length: 0\r\n\r\n"))
	}()

	client := cdp.New(&http.Client{})
	_, err = client.Evaluate(context.Background(), cdp.EvaluateRequest{
		WebSocketURL: wsURL, Expression: "1",
	})
	if err == nil {
		t.Fatal("expected error for non-upgrade response")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected error to mention 403, got: %v", err)
	}
}

func readWSFrame(t *testing.T, conn net.Conn) []byte {
	t.Helper()
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		t.Fatalf("read frame header: %v", err)
	}

	masked := header[1]&0x80 != 0
	payloadLen := int(header[1] & 0x7F)

	switch payloadLen {
	case 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(conn, ext); err != nil {
			t.Fatalf("read ext len: %v", err)
		}
		payloadLen = int(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(conn, ext); err != nil {
			t.Fatalf("read ext len: %v", err)
		}
		payloadLen = int(binary.BigEndian.Uint64(ext))
	}

	var mask []byte
	if masked {
		mask = make([]byte, 4)
		if _, err := io.ReadFull(conn, mask); err != nil {
			t.Fatalf("read mask: %v", err)
		}
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(conn, payload); err != nil {
		t.Fatalf("read payload: %v", err)
	}

	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return payload
}

func writeWSFrame(conn net.Conn, payload []byte) {
	frame := []byte{0x81}

	switch {
	case len(payload) <= 125:
		frame = append(frame, byte(len(payload)))
	case len(payload) <= 65535:
		frame = append(frame, 126)
		frame = binary.BigEndian.AppendUint16(frame, uint16(len(payload)))
	default:
		frame = append(frame, 127)
		frame = binary.BigEndian.AppendUint64(frame, uint64(len(payload)))
	}

	frame = append(frame, payload...)
	_, _ = conn.Write(frame)
}
