package utils

import (
	"net"
	"net/http"
)

// ClientIpFromRequest returns the client IP address set by the reverse proxy
// in the x-real-ip header, or nil if it is absent or invalid. The header can
// be forged by the client unless the reverse proxy overwrites it.
func ClientIpFromRequest(r *http.Request) net.IP {
	if r == nil {
		return nil
	}

	return net.ParseIP(r.Header.Get("x-real-ip"))
}
