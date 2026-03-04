//go:build js

package store

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/stregato/bao/lib/core"
)

func Open(connectionUrl string) (Store, error) {
	switch {
	case strings.HasPrefix(connectionUrl, "dav://"), strings.HasPrefix(connectionUrl, "davs://"):
		return openWebDAVFromURL(connectionUrl)

	case strings.HasPrefix(connectionUrl, "mem://"):
		return OpenMemory(connectionUrl)
	}
	return nil, core.Error(core.GenericError, "unsupported store schema on js in %s", connectionUrl)
}

func OpenWithConfig(c StoreConfig) (Store, error) {
	switch c.Type {
	case "s3":
		return OpenS3(c.Id, c.S3)
	case "webdav":
		return OpenWebDAV(c.Id, c.WebDAV)
	case "mem":
		return OpenMemory(c.Id)
	}
	return nil, core.Error(core.GenericError, "unsupported type '%s' for js store.%s", c.Type, c.Id)
}

func openWebDAVFromURL(connectionUrl string) (Store, error) {
	u, err := url.Parse(connectionUrl)
	if err != nil {
		return nil, core.Error(core.ParseError, "invalid webdav url: %v", err)
	}

	username := u.User.Username()
	password, _ := u.User.Password()
	host := u.Hostname()
	port := 0
	if u.Port() != "" {
		portVal, pErr := strconv.Atoi(u.Port())
		if pErr == nil {
			port = portVal
		}
	}

	id := host
	if u.Path != "" && u.Path != "/" {
		id = host + u.Path
	}

	config := WebDAVConfig{
		Username: username,
		Password: password,
		Host:     host,
		Port:     port,
		BasePath: u.Path,
		Query:    u.RawQuery,
		Https:    strings.HasPrefix(connectionUrl, "davs://"),
	}
	return OpenWebDAV(id, config)
}
