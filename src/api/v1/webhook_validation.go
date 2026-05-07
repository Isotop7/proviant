package v1

import (
	"context"
	"net"
	"net/url"
	"strings"
	"time"

	"codeberg.org/isotop7/proviant/errors"
)

func validateWebhookURL(rawURL string) error {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return errors.ErrWebhookURLInvalid
	}

	if parsed.Scheme != "https" {
		return errors.ErrWebhookURLNotHTTPS
	}

	host := parsed.Hostname()
	if host == "" {
		return errors.ErrWebhookURLInvalid
	}

	if ip := net.ParseIP(host); ip != nil {
		if isPrivateOrReservedIP(ip) {
			return errors.ErrWebhookURLPrivateIP
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	addrs, lookupErr := net.DefaultResolver.LookupIPAddr(ctx, host)
	// Err on the side of caution: if DNS fails, reject the URL
	if lookupErr != nil {
		return errors.ErrWebhookURLInvalid
	}

	for _, addr := range addrs {
		if isPrivateOrReservedIP(addr.IP) {
			return errors.ErrWebhookURLPrivateIP
		}
	}

	return nil
}

func isPrivateOrReservedIP(ip net.IP) bool {
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}

	// Defang IPv4-mapped IPv6 variants of private IPs (e.g. ::ffff:127.0.0.1)
	if len(ip) == net.IPv6len {
		if v4 := ip.To4(); v4 != nil {
			return isPrivateOrReservedIP(v4)
		}
	}

	// Block CG-NAT: 100.64.0.0/10 (not covered by IsPrivate)
	if _, cgnat, _ := net.ParseCIDR("100.64.0.0/10"); cgnat.Contains(ip) {
		return true
	}

	// Block carrier-grade NAT64 prefix: 64:ff9b::/96
	if strings.HasPrefix(ip.String(), "64:ff9b:") {
		return true
	}

	return false
}
