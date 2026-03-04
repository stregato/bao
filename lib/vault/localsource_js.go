//go:build js

package vault

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/fs"
	"strings"
	"time"
)

type memFileInfo struct {
	name string
	size int64
}

func (m memFileInfo) Name() string       { return m.name }
func (m memFileInfo) Size() int64        { return m.size }
func (m memFileInfo) Mode() fs.FileMode  { return 0 }
func (m memFileInfo) ModTime() time.Time { return time.Time{} }
func (m memFileInfo) IsDir() bool        { return false }
func (m memFileInfo) Sys() any           { return nil }

func parseJSBlobSource(source string) ([]byte, error) {
	if !strings.HasPrefix(source, "jsblob:") {
		return nil, fs.ErrNotExist
	}
	payload := strings.TrimPrefix(source, "jsblob:")
	if payload == "" {
		return nil, fmt.Errorf("empty jsblob payload")
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func statLocalSource(source string) (fs.FileInfo, error) {
	data, err := parseJSBlobSource(source)
	if err != nil {
		return nil, err
	}
	return memFileInfo{name: source, size: int64(len(data))}, nil
}

type memReadSeekCloser struct {
	*bytes.Reader
}

func (m *memReadSeekCloser) Close() error { return nil }

func openLocalSourceReader(source string) (readSeekCloser, error) {
	data, err := parseJSBlobSource(source)
	if err != nil {
		return nil, err
	}
	return &memReadSeekCloser{Reader: bytes.NewReader(data)}, nil
}
