//go:build js

package storage

import (
	"strings"

	"github.com/stregato/bao/lib/core"
)

// Open for JS/wasm: allow only backends that can work in browsers.
// Keep memory and WebDAV; block SFTP/Azure/Local (filesystem) and maybe S3 (CORS-sensitive).
func Open(connectionUrl string) (Store, error) {
	switch {
	case strings.HasPrefix(connectionUrl, "dav://"), strings.HasPrefix(connectionUrl, "davs://"):
		return OpenWebDAV(connectionUrl)
	case strings.HasPrefix(connectionUrl, "mem://"):
		return OpenMemory(connectionUrl)
	}
	return nil, core.Errorw("unsupported store schema on js in %s", connectionUrl)
}
