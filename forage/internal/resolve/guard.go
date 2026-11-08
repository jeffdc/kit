package resolve

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

var errBlockedAddress = errors.New("address not allowed")

var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// publicAddress reports whether ip is an ordinary internet address — not
// loopback, private (including Fly's fdaa:: network), link-local, CGNAT,
// multicast, or unspecified.
func publicAddress(ip net.IP) bool {
	return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() || cgnat.Contains(ip) ||
		(ip.To4() != nil && ip.To4()[0] == 0))
}

// guardedClient fetches arbitrary user-supplied URLs safely: the check runs
// on the address actually dialed (after DNS, and again for each redirect),
// so a hostname that resolves to an internal address is refused.
func guardedClient(allowInternal bool) *http.Client {
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
		Control: func(network, address string, _ syscall.RawConn) error {
			if allowInternal {
				return nil
			}
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || !publicAddress(ip) || (port != "80" && port != "443") {
				return fmt.Errorf("%w: %s", errBlockedAddress, address)
			}
			return nil
		},
	}
	transport := &http.Transport{
		Proxy:                  nil,
		DialContext:            dialer.DialContext,
		TLSHandshakeTimeout:    5 * time.Second,
		ResponseHeaderTimeout:  8 * time.Second,
		MaxResponseHeaderBytes: 64 << 10,
		IdleConnTimeout:        30 * time.Second,
	}
	return &http.Client{
		Timeout:   12 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("%w: scheme %s", errBlockedAddress, req.URL.Scheme)
			}
			return nil
		},
	}
}

// ctxDone is a small helper so long resolution chains stop early.
func ctxDone(ctx context.Context) bool {
	return ctx.Err() != nil
}
