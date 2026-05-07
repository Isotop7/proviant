package v1

import (
	"net"
	"testing"

	proviantErrors "codeberg.org/isotop7/proviant/errors"
)

func TestIsPrivateOrReservedIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		blocked bool
	}{
		{"public IP", "1.2.3.4", false},
		{"public IPv6", "2001:4860:4860::8888", false},
		{"loopback IPv4", "127.0.0.1", true},
		{"loopback IPv4 alt", "127.255.255.255", true},
		{"loopback IPv6", "::1", true},
		{"private 10.x", "10.0.0.1", true},
		{"private 172.16.x", "172.16.0.1", true},
		{"private 172.31.x", "172.31.255.255", true},
		{"private 192.168.x", "192.168.1.1", true},
		{"link-local 169.254.x", "169.254.1.1", true},
		{"link-local IPv6 fe80::", "fe80::1", true},
		{"unspecified IPv4", "0.0.0.0", true},
		{"unspecified IPv6", "::", true},
		{"CG-NAT 100.64.x", "100.64.0.1", true},
		{"CG-NAT 100.127.x", "100.127.255.255", true},
		{"CG-NAT near miss 100.63.x", "100.63.255.255", false},
		{"CG-NAT near miss 100.128.x", "100.128.0.1", false},
		{"IPv4-mapped loopback ::ffff:127.0.0.1", "::ffff:127.0.0.1", true},
		{"IPv4-mapped private ::ffff:10.0.0.1", "::ffff:10.0.0.1", true},
		{"NAT64 prefix 64:ff9b::1", "64:ff9b::1", true},
		{"private IPv6 fc00::", "fc00::1", true},
		{"private IPv6 fd00::", "fd00::1", true},
		{"link-local multicast 224.0.0.x", "224.0.0.1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}
			got := isPrivateOrReservedIP(ip)
			if got != tt.blocked {
				t.Errorf("isPrivateOrReservedIP(%s) = %v, want %v", tt.ip, got, tt.blocked)
			}
		})
	}
}

func TestValidateWebhookURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantError error
	}{
		{"valid HTTPS URL", "https://example.com/hook", nil},
		{"valid HTTPS with path", "https://example.com/v1/webhooks/callback", nil},
		{"HTTP scheme rejected", "http://example.com/hook", proviantErrors.ErrWebhookURLNotHTTPS},
		{"arbitrary scheme rejected", "ftp://example.com/hook", proviantErrors.ErrWebhookURLNotHTTPS},
		{"empty scheme rejected", "://example.com/hook", proviantErrors.ErrWebhookURLInvalid},
		{"no scheme rejected", "example.com/hook", proviantErrors.ErrWebhookURLInvalid},
		{"blank URL", "", proviantErrors.ErrWebhookURLInvalid},
		{"malformed URL", "https://", proviantErrors.ErrWebhookURLInvalid},
		{"localhost IP rejected", "https://127.0.0.1/hook", proviantErrors.ErrWebhookURLPrivateIP},
		{"private 10.x IP rejected", "https://10.0.0.1/hook", proviantErrors.ErrWebhookURLPrivateIP},
		{"private 192.168.x IP rejected", "https://192.168.1.1/hook", proviantErrors.ErrWebhookURLPrivateIP},
		{"IPv6 loopback rejected", "https://[::1]/hook", proviantErrors.ErrWebhookURLPrivateIP},
		{"IPv6 private rejected", "https://[fc00::1]/hook", proviantErrors.ErrWebhookURLPrivateIP},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWebhookURL(tt.url)
			if tt.wantError != nil {
				if err == nil {
					t.Errorf("validateWebhookURL(%q) = nil, want error", tt.url)
					return
				}
				if err != tt.wantError {
					t.Errorf("validateWebhookURL(%q) error = %v, want %v", tt.url, err, tt.wantError)
				}
			} else {
				if err != nil {
					t.Errorf("validateWebhookURL(%q) = %v, want nil", tt.url, err)
				}
			}
		})
	}
}
