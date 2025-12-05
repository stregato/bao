package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIdentity(t *testing.T) {
	identity, err := NewPrivateID()
	assert.NoErrorf(t, err, "cannot create identity")

	publicID := identity.PublicIDMust()
	assert.NotEmpty(t, publicID)
	assert.NotEqual(t, identity, publicID)

	cryptKey, signKey, err := DecodeID(string(identity))
	assert.NoErrorf(t, err, "cannot decode private ID")

	assert.NotEmpty(t, cryptKey)
	assert.NotEmpty(t, signKey)

	cryptKey, signKey, err = DecodeID(string(publicID))
	assert.NoErrorf(t, err, "cannot decode public ID")
	assert.NotEmpty(t, cryptKey)
	assert.NotEmpty(t, signKey)

	identity, err = NewPrivateID()
	assert.NoErrorf(t, err, "cannot create identity")

	publicID = identity.PublicIDMust()
	assert.NotEmpty(t, publicID)
	assert.NotEqual(t, identity, publicID)

}
