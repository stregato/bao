package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSignature(t *testing.T) {
	alice := NewPrivateIDMust()
	data := []byte("Hello World")

	signature, err := Sign(alice, data)
	assert.NoErrorf(t, err, "cannot sign data")

	verify := Verify(alice.PublicIDMust(), data, signature)
	assert.True(t, verify)
}
