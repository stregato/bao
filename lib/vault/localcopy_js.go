//go:build js

package vault

import (
	"bytes"
	"io"
	"sync"
)

var localCopyMem sync.Map // map[string][]byte

type memoryCopyWriter struct {
	dest string
	buf  bytes.Buffer
}

func (m *memoryCopyWriter) Write(p []byte) (int, error) {
	return m.buf.Write(p)
}

func (m *memoryCopyWriter) Close() error {
	data := append([]byte(nil), m.buf.Bytes()...)
	localCopyMem.Store(m.dest, data)
	return nil
}

func openLocalCopyWriter(dest string) (io.WriteCloser, error) {
	return &memoryCopyWriter{dest: dest}, nil
}

func cleanupLocalCopy(dest string) {
	localCopyMem.Delete(dest)
}

func ReadLocalCopyBytes(dest string) ([]byte, error) {
	v, ok := localCopyMem.Load(dest)
	if !ok {
		return nil, io.EOF
	}
	data, _ := v.([]byte)
	return append([]byte(nil), data...), nil
}

func RemoveLocalCopy(dest string) {
	cleanupLocalCopy(dest)
}
