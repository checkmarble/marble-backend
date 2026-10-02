package models

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

// Subnet is a network, (un)marshaled in CIDR notation. A bare IP address is
// accepted as a single-host network (/32 or /128).
type Subnet struct {
	net.IPNet
}

func ParseSubnet(cidr string) (Subnet, error) {
	// If a bare IP address was given
	if !strings.Contains(cidr, "/") {
		ip := net.ParseIP(cidr)

		if ip == nil {
			return Subnet{}, fmt.Errorf("invalid CIDR-less IP address %s: %w", cidr, BadParameterError)
		}

		switch {
		case ip.To4() != nil:
			cidr = ip.String() + "/32"
		case ip.To16() != nil:
			cidr = ip.String() + "/128"
		default:
			return Subnet{}, fmt.Errorf("invalid CIDR-less IP address %s: %w", cidr, BadParameterError)
		}
	}

	_, subnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return Subnet{}, fmt.Errorf("%w: %w", err, BadParameterError)
	}

	return Subnet{*subnet}, nil
}

func (s *Subnet) UnmarshalJSON(b []byte) error {
	var cidr string

	if err := json.Unmarshal(b, &cidr); err != nil {
		return err
	}

	subnet, err := ParseSubnet(cidr)
	if err != nil {
		return err
	}

	*s = subnet

	return nil
}

func (s Subnet) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}
