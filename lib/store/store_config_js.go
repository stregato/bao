//go:build js

package store

type StoreConfig struct {
	Id     string       `json:"id" yaml:"id"`
	Type   string       `json:"type" yaml:"type"`
	S3     S3Config     `json:"s3" yaml:"s3"`
	WebDAV WebDAVConfig `json:"webdav" yaml:"webdav"`
}
