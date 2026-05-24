package httpclient

import (
	"net"
	"net/http"
	"time"
)

// New returns an HTTP client with conservative timeouts for local services.
func New() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
			IdleConnTimeout:       90 * time.Second,
			MaxIdleConns:          10,
		},
	}
}

// Close shuts down idle connections on the client's transport.
func Close(c *http.Client) {
	if c == nil {
		return
	}
	t, ok := c.Transport.(*http.Transport)
	if !ok {
		return
	}
	t.CloseIdleConnections()
}
