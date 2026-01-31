package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIdentity(t *testing.T) {
	idSecret, err := NewPrivateID()
	assert.NoErrorf(t, err, "cannot create identity")

	id := idSecret.PublicIDMust()
	assert.NotEmpty(t, id)
	assert.NotEqual(t, idSecret, id)

	cryptKey, signKey, err := idSecret.Decode()
	assert.NoErrorf(t, err, "cannot decode private ID")

	assert.NotEmpty(t, cryptKey)
	assert.NotEmpty(t, signKey)

	cryptKey, signKey, err = id.Decode()
	assert.NoErrorf(t, err, "cannot decode public ID")
	assert.NotEmpty(t, cryptKey)
	assert.NotEmpty(t, signKey)

	idSecret, err = NewPrivateID()
	assert.NoErrorf(t, err, "cannot create identity")

	id = idSecret.PublicIDMust()
	assert.NotEmpty(t, id)
	assert.NotEqual(t, idSecret, id)

}
