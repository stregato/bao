package vault

import (
	"net"
	"net/url"
	"strings"
	"testing"
	"time"
)

func requireSyncRelay(t *testing.T, relayURL string) {
	t.Helper()
	u, err := url.Parse(relayURL)
	if err != nil || u.Host == "" {
		t.Skipf("invalid sync relay url %q: %v", relayURL, err)
	}

	hostPort := u.Host
	if !strings.Contains(hostPort, ":") {
		if u.Scheme == "wss" {
			hostPort += ":443"
		} else {
			hostPort += ":80"
		}
	}

	conn, err := net.DialTimeout("tcp", hostPort, 2*time.Second)
	if err != nil {
		t.Skipf("sync relay unavailable at %s: %v", hostPort, err)
	}
	_ = conn.Close()
}
