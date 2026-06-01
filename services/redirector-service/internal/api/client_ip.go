package api

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

func ParseTrustedProxyCIDRs(value string) ([]netip.Prefix, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	parts := strings.Split(value, ",")
	prefixes := make([]netip.Prefix, 0, len(parts))
	for _, part := range parts {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("parse trusted proxy CIDR %q: %w", part, err)
		}
		prefixes = append(prefixes, prefix.Masked())
	}

	return prefixes, nil
}

func clientIPAddress(r *http.Request, trustedProxyCIDRs []netip.Prefix) string {
	remoteIP, ok := parseIPAddress(r.RemoteAddr)
	if !ok {
		return r.RemoteAddr
	}
	if !isTrustedProxy(remoteIP, trustedProxyCIDRs) {
		return remoteIP.String()
	}

	if forwardedIP, ok := clientIPFromXForwardedFor(r.Header.Get("X-Forwarded-For"), trustedProxyCIDRs); ok {
		return forwardedIP.String()
	}
	if realIP, ok := parseIPAddress(r.Header.Get("X-Real-Ip")); ok {
		return realIP.String()
	}

	return remoteIP.String()
}

func clientIPFromXForwardedFor(value string, trustedProxyCIDRs []netip.Prefix) (netip.Addr, bool) {
	if strings.TrimSpace(value) == "" {
		return netip.Addr{}, false
	}

	parts := strings.Split(value, ",")
	var leftmostIP netip.Addr
	for i := len(parts) - 1; i >= 0; i-- {
		ip, ok := parseIPAddress(parts[i])
		if !ok {
			return netip.Addr{}, false
		}
		leftmostIP = ip
		if !isTrustedProxy(ip, trustedProxyCIDRs) {
			return ip, true
		}
	}

	return leftmostIP, leftmostIP.IsValid()
}

func parseIPAddress(value string) (netip.Addr, bool) {
	value = strings.TrimSpace(value)
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}

	ip, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Addr{}, false
	}
	return ip.Unmap(), true
}

func isTrustedProxy(ip netip.Addr, trustedProxyCIDRs []netip.Prefix) bool {
	for _, prefix := range trustedProxyCIDRs {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}
