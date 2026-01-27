//go:build !js
// +build !js

package vault

import (
	"crypto/sha1"
	"net"

	"github.com/stregato/bao/lib/core"
)

// getNodeHash returns a stable 8-bit hash derived from the first available MAC address,
// falling back to the system hostname. This implementation is used on all platforms
// except WebAssembly/JavaScript, where net operations are restricted.
func getNodeHash() uint8 {
	core.Start("getting node hash")
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			if len(iface.HardwareAddr) > 0 {
				h := sha1.Sum(iface.HardwareAddr)
				core.End("successfully retrieved node hash %d", h[0])
				return h[0]
			}
		}
	}
	// Fallback to hostname (best-effort). If it fails, use a constant non-zero value.
	hosts, _ := net.LookupHost("localhost")
	var host string
	if len(hosts) > 0 {
		host = hosts[0]
	}
	h := sha1.Sum([]byte(host))
	core.End("successfully retrieved node hash %d", h[0])
	return h[0]
}
