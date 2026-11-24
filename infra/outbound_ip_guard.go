package infra

import (
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"

	"github.com/cockroachdb/errors"
)

var ErrReservedIP = errors.New("destination IP address is reserved")

// Reserved IP ranges that should never be outbound targets (unless whitelisted)
var reservedIPBlocks []*net.IPNet

func init() {
	// Comprehensive list of reserved/special-use IP ranges
	// References: RFC 5735, RFC 6890, IANA Special-Purpose Address Registries
	reserved := []string{
		// IPv4 reserved ranges
		"0.0.0.0/8",       // "This network" - includes 0.0.0.0 which routes to localhost
		"10.0.0.0/8",      // Private (RFC1918)
		"100.64.0.0/10",   // CGNAT - Carrier-grade NAT (RFC 6598)
		"127.0.0.0/8",     // Loopback
		"169.254.0.0/16",  // Link-local (includes cloud metadata 169.254.169.254)
		"172.16.0.0/12",   // Private (RFC1918)
		"192.0.0.0/24",    // IETF Protocol Assignments
		"192.0.2.0/24",    // TEST-NET-1 - Documentation (RFC 5737)
		"192.88.99.0/24",  // 6to4 Relay Anycast (deprecated)
		"192.168.0.0/16",  // Private (RFC1918)
		"198.18.0.0/15",   // Benchmark testing (RFC 2544)
		"198.51.100.0/24", // TEST-NET-2 - Documentation (RFC 5737)
		"203.0.113.0/24",  // TEST-NET-3 - Documentation (RFC 5737)
		"224.0.0.0/4",     // Multicast
		"240.0.0.0/4",     // Reserved for future use (includes broadcast)

		// IPv6 reserved ranges
		"::/128",  // Unspecified address
		"::1/128", // Loopback
		// Note: ::ffff:0:0/96 (IPv4-mapped) is NOT blocked because Go internally
		// represents IPv4 as IPv4-mapped IPv6 and we already check IPv4 ranges above.
		"64:ff9b::/96",  // IPv4/IPv6 translation (RFC 6052)
		"100::/64",      // Discard prefix (RFC 6666)
		"2001::/32",     // Teredo tunneling
		"2001:db8::/32", // Documentation (RFC 3849)
		"2002::/16",     // 6to4 (deprecated)
		"fc00::/7",      // Unique local addresses
		"fe80::/10",     // Link-local
		"ff00::/8",      // Multicast
	}

	for _, cidr := range reserved {
		_, block, err := net.ParseCIDR(cidr)
		if err == nil {
			reservedIPBlocks = append(reservedIPBlocks, block)
		}
	}
}

// ParseCIDRList parses a comma-separated list of CIDR ranges.
// Invalid CIDRs are silently ignored.
func ParseCIDRList(cidrList string) []*net.IPNet {
	if cidrList == "" {
		return nil
	}

	var blocks []*net.IPNet
	for _, cidr := range strings.Split(cidrList, ",") {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		_, block, err := net.ParseCIDR(cidr)
		if err == nil {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

// IsReservedIP reports whether ip is in a reserved range and not covered by the whitelist.
func IsReservedIP(ip net.IP, whitelist []*net.IPNet) bool {
	// Check whitelist first - if IP is whitelisted, it's not considered reserved
	for _, block := range whitelist {
		if block.Contains(ip) {
			return false
		}
	}

	// Check standard library helpers
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}

	// Check our extended reserved blocks
	for _, block := range reservedIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// NewGuardedTransport returns an HTTP transport that refuses to connect to reserved IP addresses
// (unless whitelisted). The check runs on the address actually dialed, after DNS resolution, so it
// also covers redirects and DNS changes. If an HTTP proxy is configured, the proxy's address is
// the one checked.
func NewGuardedTransport(ipWhitelist []*net.IPNet) *http.Transport {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{Proxy: http.ProxyFromEnvironment}
	}
	transport := base.Clone()
	transport.DialContext = (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if (network != "tcp4" && network != "tcp6") || ip == nil || IsReservedIP(ip, ipWhitelist) {
				return ErrReservedIP
			}
			return nil
		},
	}).DialContext
	return transport
}
