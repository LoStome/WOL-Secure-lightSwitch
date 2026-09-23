package auth

import (
	"fmt"
	"testing"
)

func TestParseTrustedProxies(t *testing.T) {
	proxies, err := ParseTrustedProxies(" 127.0.0.1, 10.0.0.0/8, ::1 ")
	if err != nil {
		t.Fatalf("parse valid trusted proxies: %v", err)
	}
	want := []string{"127.0.0.1", "10.0.0.0/8", "::1"}
	if fmt.Sprint(proxies) != fmt.Sprint(want) {
		t.Fatalf("trusted proxies = %v, want %v", proxies, want)
	}

	for _, value := range []string{"not-an-address", "127.0.0.1,,::1", "0.0.0.0/0", "::/0"} {
		if _, err := ParseTrustedProxies(value); err == nil {
			t.Errorf("ParseTrustedProxies(%q) succeeded, want error", value)
		}
	}
}
