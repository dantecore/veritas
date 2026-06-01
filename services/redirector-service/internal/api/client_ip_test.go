package api

import (
	"net/http/httptest"
	"testing"
)

func TestClientIPAddress(t *testing.T) {
	trustedProxyCIDRs, err := ParseTrustedProxyCIDRs("172.28.0.2/32,10.0.0.0/8")
	if err != nil {
		t.Fatalf("parse trusted proxy CIDRs: %v", err)
	}

	tests := []struct {
		name       string
		remoteAddr string
		forwarded  string
		realIP     string
		want       string
	}{
		{
			name:       "strips port from direct client",
			remoteAddr: "192.0.2.1:1234",
			want:       "192.0.2.1",
		},
		{
			name:       "ignores spoofed headers from direct client",
			remoteAddr: "192.0.2.1:1234",
			forwarded:  "198.51.100.1",
			realIP:     "198.51.100.2",
			want:       "192.0.2.1",
		},
		{
			name:       "uses forwarded client from trusted proxy",
			remoteAddr: "172.28.0.2:43120",
			forwarded:  "192.0.2.1",
			want:       "192.0.2.1",
		},
		{
			name:       "walks forwarded chain from trusted side",
			remoteAddr: "172.28.0.2:43120",
			forwarded:  "198.51.100.1, 192.0.2.1, 10.0.0.5",
			want:       "192.0.2.1",
		},
		{
			name:       "falls back to real IP for malformed forwarded header",
			remoteAddr: "172.28.0.2:43120",
			forwarded:  "unknown",
			realIP:     "192.0.2.1",
			want:       "192.0.2.1",
		},
		{
			name:       "supports direct IPv6 client",
			remoteAddr: "[2001:db8::1]:1234",
			want:       "2001:db8::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			req.Header.Set("X-Forwarded-For", tt.forwarded)
			req.Header.Set("X-Real-Ip", tt.realIP)

			if got := clientIPAddress(req, trustedProxyCIDRs); got != tt.want {
				t.Fatalf("expected IP address %q, got %q", tt.want, got)
			}
		})
	}
}

func TestParseTrustedProxyCIDRs(t *testing.T) {
	prefixes, err := ParseTrustedProxyCIDRs(" 172.28.0.2/32, 10.0.1.8/8 ")
	if err != nil {
		t.Fatalf("parse trusted proxy CIDRs: %v", err)
	}
	if len(prefixes) != 2 || prefixes[0].String() != "172.28.0.2/32" || prefixes[1].String() != "10.0.0.0/8" {
		t.Fatalf("unexpected trusted proxy CIDRs: %#v", prefixes)
	}

	if _, err := ParseTrustedProxyCIDRs("not-a-cidr"); err == nil {
		t.Fatal("expected invalid CIDR error")
	}
}
