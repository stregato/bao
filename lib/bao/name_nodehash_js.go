//go:build js
// +build js

package bao

import (
	"crypto/sha1"
	"syscall/js"
)

// getNodeHash for JavaScript/WASM environments avoids net lookups.
// It derives a host-dependent, but offline-safe, 8-bit hash from window.location.host
// or from navigator.userAgent as a fallback.
func getNodeHash() uint8 {
	var basis string
	// Prefer window.location.host when available
	if js.Global().Get("window").Truthy() {
		loc := js.Global().Get("window").Get("location")
		if loc.Truthy() {
			h := loc.Get("host")
			if h.Truthy() {
				basis = h.String()
			}
		}
	}
	// Fallback to self.location (e.g., in workers) or navigator.userAgent
	if basis == "" {
		if js.Global().Get("self").Truthy() {
			loc := js.Global().Get("self").Get("location")
			if loc.Truthy() {
				h := loc.Get("host")
				if h.Truthy() {
					basis = h.String()
				}
			}
		}
	}
	if basis == "" && js.Global().Get("navigator").Truthy() {
		ua := js.Global().Get("navigator").Get("userAgent")
		if ua.Truthy() {
			basis = ua.String()
		}
	}
	// As last resort, use a fixed string to remain deterministic but non-zero.
	if basis == "" {
		basis = "wasm"
	}
	sum := sha1.Sum([]byte(basis))
	return sum[0]
}
