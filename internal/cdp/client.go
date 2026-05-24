package cdp

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

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

func eval(ctx context.Context, wsURL string, js string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := wsDial(ctx, wsURL)
	if err != nil {
		return "", fmt.Errorf("cdp dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	req := cdpRequest{
		ID:     1,
		Method: "Runtime.evaluate",
		Params: cdpParams{
			Expression:    js,
			ReturnByValue: true,
		},
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("cdp marshal: %w", err)
	}

	if err := wsWrite(conn, payload); err != nil {
		return "", fmt.Errorf("cdp write: %w", err)
	}

	respPayload, err := wsRead(conn)
	if err != nil {
		return "", fmt.Errorf("cdp read: %w", err)
	}

	var resp cdpResponse
	if err := json.Unmarshal(respPayload, &resp); err != nil {
		return "", fmt.Errorf("cdp unmarshal: %w", err)
	}

	return resp.Result.Result.Value, nil
}

func wsDial(ctx context.Context, rawURL string) (net.Conn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	host := u.Host
	if u.Port() == "" {
		host += ":80"
	}

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, err
	}

	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		_ = conn.Close()
		return nil, err
	}

	reqPath := u.RequestURI()
	handshake := "GET " + reqPath + " HTTP/1.1\r\n" +
		"Host: " + u.Host + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + base64.StdEncoding.EncodeToString(key) + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n" +
		"\r\n"

	if _, err := conn.Write([]byte(handshake)); err != nil {
		_ = conn.Close()
		return nil, err
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		_ = conn.Close()
		return nil, fmt.Errorf("websocket upgrade failed: %d", resp.StatusCode)
	}

	return conn, nil
}

func wsWrite(conn net.Conn, payload []byte) error {
	mask := make([]byte, 4)
	if _, err := rand.Read(mask); err != nil {
		return err
	}

	frame := make([]byte, 0, 14+len(payload))
	frame = append(frame, 0x81) // FIN + text opcode

	switch {
	case len(payload) <= 125:
		frame = append(frame, byte(len(payload))|0x80) // masked
	case len(payload) <= 65535:
		frame = append(frame, 126|0x80)
		frame = binary.BigEndian.AppendUint16(frame, uint16(len(payload)))
	default:
		frame = append(frame, 127|0x80)
		frame = binary.BigEndian.AppendUint64(frame, uint64(len(payload)))
	}

	frame = append(frame, mask...)

	masked := make([]byte, len(payload))
	for i, b := range payload {
		masked[i] = b ^ mask[i%4]
	}
	frame = append(frame, masked...)

	_, err := conn.Write(frame)
	return err
}

func wsRead(conn net.Conn) ([]byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}

	payloadLen := int(header[1] & 0x7F)
	switch payloadLen {
	case 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(conn, ext); err != nil {
			return nil, err
		}
		payloadLen = int(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(conn, ext); err != nil {
			return nil, err
		}
		payloadLen = int(binary.BigEndian.Uint64(ext))
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return nil, err
	}

	return payload, nil
}
