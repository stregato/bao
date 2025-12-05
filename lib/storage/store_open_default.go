//go:build !js

package storage

import (
	"strings"

	"github.com/stregato/bao/lib/core"
)

// Open creates a new exchanger given a provided configuration (non-JS platforms)
func Open(connectionUrl string) (Store, error) {
	switch {
	case strings.HasPrefix(connectionUrl, "sftp://"):
		return OpenSFTP(connectionUrl)
	case strings.HasPrefix(connectionUrl, "s3://"):
		return OpenS3(connectionUrl)
	case strings.HasPrefix(connectionUrl, "file:/"):
		return OpenLocal(connectionUrl)
	case strings.HasPrefix(connectionUrl, "dav://"):
		return OpenWebDAV(connectionUrl)
	case strings.HasPrefix(connectionUrl, "davs://"):
		return OpenWebDAV(connectionUrl)
	case strings.HasPrefix(connectionUrl, "mem://"):
		return OpenMemory(connectionUrl)
	}
	return nil, core.Errorw("unsupported store schema in %s", connectionUrl)
}
