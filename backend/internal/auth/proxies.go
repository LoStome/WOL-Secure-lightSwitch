package auth

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

func ParseTrustedProxies(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	parts := strings.Split(value, ",")
	proxies := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, errors.New("TRUSTED_PROXIES contains an empty entry")
		}

		canonical := ""
		if strings.Contains(part, "/") {
			prefix, err := netip.ParsePrefix(part)
			if err != nil {
				return nil, fmt.Errorf("invalid trusted proxy %q: %w", part, err)
			}
			if prefix.Bits() == 0 {
				return nil, fmt.Errorf("trusted proxy %q permits every address", part)
			}
			canonical = prefix.Masked().String()
		} else {
			address, err := netip.ParseAddr(part)
			if err != nil || address.Zone() != "" {
				return nil, fmt.Errorf("invalid trusted proxy %q", part)
			}
			canonical = address.String()
		}

		if _, duplicate := seen[canonical]; duplicate {
			continue
		}
		seen[canonical] = struct{}{}
		proxies = append(proxies, canonical)
	}
	return proxies, nil
}
