package security

import (
	"testing"

	"github.com/stregato/bao/lib/core"
	"github.com/stretchr/testify/assert"
)

func TestEccrypt(t *testing.T) {
	alice := NewPrivateIDMust()
	data := core.GenerateRandomBytes(32)

	encrypted, err := EcEncrypt(alice.PublicIDMust(), data)
	assert.NoError(t, err)

	decrypted, err := EcDecrypt(alice, encrypted)
	assert.NoError(t, err)

	assert.Equal(t, data, decrypted)
}
