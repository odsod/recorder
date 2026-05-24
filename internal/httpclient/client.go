package httpclient

import (
	"net"
	"net/http"
	"time"
)

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
