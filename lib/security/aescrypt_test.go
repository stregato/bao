package security

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"

	"github.com/stregato/bao/lib/core"
	"github.com/stretchr/testify/assert"
)

func TestAESCrypt(t *testing.T) {

	b := make([]byte, 1024)
	rand.Read(b)

	b = []byte("Hello")
	key := GenerateBytesKey(32)
	keyFunc := func(_ uint64) []byte {
		return key
	}

	r := core.NewBytesReader(b)
	er, _ := EncryptingReader(0, keyFunc, r)
	w := &bytes.Buffer{}

	io.Copy(w, er)

	w2 := &bytes.Buffer{}
	ew, _ := DecryptingWriter(keyFunc, w2)
	io.Copy(ew, w)

	assert.Equal(t, w2.Bytes(), b)
}
