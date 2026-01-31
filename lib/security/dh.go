package security

import (
	"crypto/sha256"
	"fmt"

	eciesgo "github.com/ecies/go/v2"
	"github.com/stregato/bao/lib/core"
)

func DiffieHellmanKey(privateID PrivateID, publicID PublicID) ([]byte, error) {
	privateKey, _, err := privateID.Decode()
	if err != nil {
		return nil, core.Error(core.ParseError, "cannot decode keys", err)
	}

	publicKey, _, err := publicID.Decode()
	if err != nil {
		return nil, core.Error(core.ParseError, "cannot decode keys", err)
	}

	pr := eciesgo.NewPrivateKeyFromBytes(privateKey)
	if pr == nil {
		return nil, fmt.Errorf("cannot convert bytes to secp256k1 private key")
	}

	pu, err := eciesgo.NewPublicKeyFromBytes(publicKey)
	if err != nil {
		return nil, core.Error(core.GenericError, "cannot convert bytes to secp256k1 public key", err)
	}

	data, err := pr.ECDH(pu)
	if err != nil {
		return nil, core.Error(core.GenericError, "cannot perform ECDH", err)
	}

	h := sha256.Sum256(data)
	return h[:], nil
}
