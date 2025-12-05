package core

import (
	"bytes"
	"compress/gzip"
	"io"
)

func GzipCompress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, err := w.Write(data)
	if err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func GzipDecompress(data []byte) ([]byte, error) {
	Trace("FUNC_START[GzipDecompress]: decompressing data of size %d", len(data))
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	Trace("FUNC_END[GzipDecompress]: successfully decompressed data to size %d", len(out))
	return out, nil
}
