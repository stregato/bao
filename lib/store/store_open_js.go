//go:build js

package store

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/stregato/bao/lib/core"
)

// Open for JS/wasm: allow only backends that can work in browsers.
// Keep memory and WebDAV; block SFTP/Azure/Local (filesystem) and maybe S3 (CORS-sensitive).
func Open(connectionUrl string) (Store, error) {
	switch {
	case strings.HasPrefix(connectionUrl, "dav://"), strings.HasPrefix(connectionUrl, "davs://"):
		// Parse WebDAV URL: dav://user:pass@host:port/path or davs://user:pass@host:port/path
		u, err := url.Parse(connectionUrl)
		if err != nil {
			return nil, core.Errorw(core.ParseError, "invalid webdav url: %v", err)
		}

		// Extract credentials
		username := u.User.Username()
		password, _ := u.User.Password()

		// Extract host and port
		host := u.Hostname()
		port := 0
		if u.Port() != "" {
			// Parse port if present
			portVal, err := strconv.Atoi(u.Port())
			if err == nil {
				port = portVal
			}
		}

		// Build ID from host and path
		id := host
		if u.Path != "" && u.Path != "/" {
			id = host + u.Path
		}

		// Determine if HTTPS based on scheme
		https := strings.HasPrefix(connectionUrl, "davs://")

		config := WebDAVConfig{
			Username: username,
			Password: password,
			Host:     host,
			Port:     port,
			BasePath: u.Path,
			Https:    https,
		}

		return OpenWebDAV(id, config)

	case strings.HasPrefix(connectionUrl, "mem://"):
		return OpenMemory(connectionUrl)
	}
	return nil, core.Errorw(core.GenericError, "unsupported store schema on js in %s", connectionUrl)
}
