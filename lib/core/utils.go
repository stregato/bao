package core

import (
	"encoding/base64"
	"encoding/binary"
	"path"

	"github.com/dgryski/go-farm"
	"github.com/ethereum/go-ethereum/crypto/blake2b"
)

func Map[K comparable, T any](s []T, f func(T) K) map[K]T {
	result := make(map[K]T)

	for _, v := range s {
		result[f(v)] = v
	}
	return result
}

func Keys[T comparable, U any](s map[T]U) []T {
	var result []T
	for v := range s {
		result = append(result, v)
	}
	return result
}

func Values[T comparable, U any](s map[T]U) []U {
	var result []U
	for _, v := range s {
		result = append(result, v)
	}
	return result
}

func Contains[T comparable](s []T, v T) bool {
	for _, e := range s {
		if e == v {
			return true
		}
	}
	return false
}

func SplitPath(p string) (string, string) {
	dir, base := path.Split(p)
	dir = path.Clean(dir)
	if dir == "." {
		dir = ""
	}
	return dir, base
}

func Dir(p string) string {
	dir := path.Dir(p)
	dir = path.Clean(dir)
	if dir == "." {
		dir = ""
	}
	return dir
}

func Uint64ToBytes(n uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, n)
	return b
}

// Int64Hash generates a 64-bit hash where the highest bit is set to 0.
func Int64Hash(data []byte) uint64 {
	return (farm.Hash64(data) &^ (1 << 63)) | (1 << 62)
}

// BigHash generates a 512-bit hash using blake2b
func BigHash(data []byte) []byte {
	h, _ := blake2b.New512(nil)
	h.Write(data)
	return h.Sum(nil)
}

// StringHash generates a 512-bit hash and encode in url safe base64.
func StringHash(data []byte) string {
	hash := BigHash(data)
	return base64.RawURLEncoding.EncodeToString(hash)
}

func SipHash(data []byte) uint64 {
	return farm.Hash64(data) &^ (1 << 63)
}

// DefaultIfZero returns the default value if v is zero value for type T.
func DefaultIfZero[T comparable](v, def T) T {
	if v == *new(T) {
		return def
	}
	return v
}
