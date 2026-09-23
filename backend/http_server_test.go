package main

import (
	"net/http"
	"testing"
)

func TestNewHTTPServerConfiguresTimeouts(t *testing.T) {
	server := newHTTPServer(http.NotFoundHandler(), "127.0.0.1:7500")

	if server.ReadHeaderTimeout <= 0 {
		t.Error("ReadHeaderTimeout must be configured")
	}
	if server.ReadTimeout <= 0 {
		t.Error("ReadTimeout must be configured")
	}
	if server.WriteTimeout <= 0 {
		t.Error("WriteTimeout must be configured")
	}
	if server.IdleTimeout <= 0 {
		t.Error("IdleTimeout must be configured")
	}
	if server.Addr != "127.0.0.1:7500" {
		t.Errorf("Addr = %q, want %q", server.Addr, "127.0.0.1:7500")
	}
	if server.Handler == nil {
		t.Fatal("Handler must be configured")
	}
}
