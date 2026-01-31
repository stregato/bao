//go:build !js

package store

import (
	"github.com/stregato/bao/lib/core"
)

type StoreConfig struct {
	Id     string       `json:"id"`     // Storage identifier
	Type   string       `json:"type"`   // Storage type: s3, sftp, file, dav, mem, relay
	S3     S3Config     `json:"s3"`     // S3 specific configuration
	SFTP   SFTPConfig   `json:"sftp"`   // SFTP specific configuration
	Azure  AzureConfig  `json:"azure"`  // Azure specific configuration
	Local  LocalConfig  `json:"local"`  // Local specific configuration
	WebDAV WebDAVConfig `json:"webdav"` // WebDAV specific configuration
	Relay  RelayConfig  `json:"relay"`  // Relay specific configuration
}

// Open creates a new exchanger given a provided configuration (non-JS platforms)
func Open(c StoreConfig) (Store, error) {
	switch c.Type {
	case "sftp":
		return OpenSFTP(c.Id, c.SFTP)
	case "s3":
		return OpenS3(c.Id, c.S3)
	case "azure":
		return OpenAzure(c.Id, c.Azure)
	case "local":
		return OpenLocal(c.Id, c.Local)
	case "webdav":
		return OpenWebDAV(c.Id, c.WebDAV)
	case "mem":
		return OpenMemory(c.Id)
	case "relay":
		return OpenRelay(c.Id, c.Relay)
	}
	return nil, core.Error(core.GenericError, "unsupported type '%s' for store.%s", c.Type, c.Id)
}
