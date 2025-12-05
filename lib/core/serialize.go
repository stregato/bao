package core

import (
	"encoding/binary"
	"io"
)

// func EncodeBinary(data []byte) string {
// 	s := base64.StdEncoding.EncodeToString(data)
// 	s = strings.ReplaceAll(s, "/", "_")
// 	s = strings.ReplaceAll(s, "=", "!")
// 	return s
// }

// func DecodeBinary(data string) ([]byte, error) {
// 	data = strings.ReplaceAll(data, "!", "=")
// 	data = strings.ReplaceAll(data, "_", "/")
// 	return base64.StdEncoding.DecodeString(data)
// }

func WriteBytes(data []byte, w io.Writer) error {
	lenB := make([]byte, 4)
	binary.BigEndian.PutUint32(lenB, uint32(len(data)))
	n, err := w.Write(lenB)
	if err == nil && n != len(lenB) {
		return io.ErrShortWrite
	}
	if err != nil {
		return Errorw("cannot write data to stream", err)
	}

	n, err = w.Write(data)
	if err == nil && n != len(data) {
		return io.ErrShortWrite
	}
	if err != nil {
		return Errorw("cannot write data to stream", err)
	}
	return err
}

func ReadBytes(r io.Reader) ([]byte, error) {
	lenB := make([]byte, 4)
	n, err := r.Read(lenB)
	if err != nil {
		return nil, Errorw("cannot read data from stream", err)
	}
	if n != len(lenB) {
		return nil, io.ErrNoProgress
	}

	data := make([]byte, binary.BigEndian.Uint32(lenB))
	n = 0
	cnt := 4 + len(data)/64
	for n < len(data) {
		m, err := r.Read(data[n:])
		if err != nil {
			return nil, Errorw("cannot read data from stream", err)
		}
		n += m
		cnt--
		if cnt == 0 {
			return nil, io.ErrNoProgress
		}
	}

	return data, nil
}
