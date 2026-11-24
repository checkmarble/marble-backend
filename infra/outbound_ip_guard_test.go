package infra

import (
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCIDRList(t *testing.T) {
	t.Run("empty string returns nil", func(t *testing.T) {
		result := ParseCIDRList("")
		assert.Nil(t, result)
	})

	t.Run("parses single CIDR", func(t *testing.T) {
		result := ParseCIDRList("10.0.0.0/8")
		assert.Len(t, result, 1)
	})

	t.Run("parses multiple CIDRs", func(t *testing.T) {
		result := ParseCIDRList("10.0.0.0/8, 192.168.0.0/16, 172.16.0.0/12")
		assert.Len(t, result, 3)
	})

	t.Run("ignores invalid CIDRs", func(t *testing.T) {
		result := ParseCIDRList("10.0.0.0/8, invalid, 192.168.0.0/16")
		assert.Len(t, result, 2)
	})

	t.Run("handles extra whitespace", func(t *testing.T) {
		result := ParseCIDRList("  10.0.0.0/8  ,  192.168.0.0/16  ")
		assert.Len(t, result, 2)
	})
}

func TestIsReservedIP(t *testing.T) {
	t.Run("loopback is reserved", func(t *testing.T) {
		assert.True(t, IsReservedIP(net.ParseIP("127.0.0.1"), nil))
		assert.True(t, IsReservedIP(net.ParseIP("127.255.255.255"), nil))
	})

	t.Run("private ranges are reserved", func(t *testing.T) {
		// 10.0.0.0/8
		assert.True(t, IsReservedIP(net.ParseIP("10.0.0.1"), nil))
		assert.True(t, IsReservedIP(net.ParseIP("10.255.255.255"), nil))

		// 172.16.0.0/12
		assert.True(t, IsReservedIP(net.ParseIP("172.16.0.1"), nil))
		assert.True(t, IsReservedIP(net.ParseIP("172.31.255.255"), nil))

		// 192.168.0.0/16
		assert.True(t, IsReservedIP(net.ParseIP("192.168.0.1"), nil))
		assert.True(t, IsReservedIP(net.ParseIP("192.168.255.255"), nil))
	})

	t.Run("link-local is reserved (cloud metadata)", func(t *testing.T) {
		assert.True(t, IsReservedIP(net.ParseIP("169.254.169.254"), nil))
		assert.True(t, IsReservedIP(net.ParseIP("169.254.0.1"), nil))
	})

	t.Run("CGNAT range is reserved", func(t *testing.T) {
		assert.True(t, IsReservedIP(net.ParseIP("100.64.0.1"), nil))
		assert.True(t, IsReservedIP(net.ParseIP("100.127.255.255"), nil))
	})

	t.Run("TEST-NET ranges are reserved", func(t *testing.T) {
		// TEST-NET-1
		assert.True(t, IsReservedIP(net.ParseIP("192.0.2.1"), nil))
		// TEST-NET-2
		assert.True(t, IsReservedIP(net.ParseIP("198.51.100.1"), nil))
		// TEST-NET-3
		assert.True(t, IsReservedIP(net.ParseIP("203.0.113.1"), nil))
	})

	t.Run("benchmark range is reserved", func(t *testing.T) {
		assert.True(t, IsReservedIP(net.ParseIP("198.18.0.1"), nil))
		assert.True(t, IsReservedIP(net.ParseIP("198.19.255.255"), nil))
	})

	t.Run("0.0.0.0/8 is reserved", func(t *testing.T) {
		assert.True(t, IsReservedIP(net.ParseIP("0.0.0.0"), nil))
		assert.True(t, IsReservedIP(net.ParseIP("0.0.0.1"), nil))
	})

	t.Run("multicast is reserved", func(t *testing.T) {
		assert.True(t, IsReservedIP(net.ParseIP("224.0.0.1"), nil))
		assert.True(t, IsReservedIP(net.ParseIP("239.255.255.255"), nil))
	})

	t.Run("public IPs are not reserved", func(t *testing.T) {
		assert.False(t, IsReservedIP(net.ParseIP("8.8.8.8"), nil))
		assert.False(t, IsReservedIP(net.ParseIP("1.1.1.1"), nil))
		assert.False(t, IsReservedIP(net.ParseIP("142.250.185.14"), nil)) // google.com
	})

	t.Run("whitelist overrides reserved", func(t *testing.T) {
		whitelist := ParseCIDRList("10.0.0.0/8")

		// 10.x should now be allowed
		assert.False(t, IsReservedIP(net.ParseIP("10.0.0.1"), whitelist))
		assert.False(t, IsReservedIP(net.ParseIP("10.255.255.255"), whitelist))

		// Other private ranges still blocked
		assert.True(t, IsReservedIP(net.ParseIP("192.168.1.1"), whitelist))
	})

	t.Run("IPv6 loopback is reserved", func(t *testing.T) {
		assert.True(t, IsReservedIP(net.ParseIP("::1"), nil))
	})

	t.Run("IPv6 link-local is reserved", func(t *testing.T) {
		assert.True(t, IsReservedIP(net.ParseIP("fe80::1"), nil))
	})

	t.Run("IPv6 unique local is reserved", func(t *testing.T) {
		assert.True(t, IsReservedIP(net.ParseIP("fd00::1"), nil))
	})
}

func TestGuardedTransport(t *testing.T) {
	var reached atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached.Add(1) }))
	defer server.Close()

	t.Run("blocks reserved IP", func(t *testing.T) {
		reached.Store(0)
		client := &http.Client{Transport: NewGuardedTransport(nil)}
		_, err := client.Get(server.URL)
		require.ErrorIs(t, err, ErrReservedIP)
		assert.Zero(t, reached.Load())
	})

	t.Run("allows whitelisted IP", func(t *testing.T) {
		reached.Store(0)
		client := &http.Client{Transport: NewGuardedTransport(ParseCIDRList("127.0.0.0/8"))}
		resp, err := client.Get(server.URL)
		require.NoError(t, err)
		resp.Body.Close()
		assert.EqualValues(t, 1, reached.Load())
	})
}
