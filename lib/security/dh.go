package security

import (
	"crypto/sha256"
	"fmt"

	eciesgo "github.com/ecies/go/v2"
	"github.com/stregato/bao/lib/core"
)

func DiffieHellmanKey(privateID PrivateID, publicID PublicID) ([]byte, error) {
	privateKey, _, err := DecodeID(string(privateID))
	if err != nil {
		return nil, core.Errorw("cannot decode keys", err)
	}

	publicKey, _, err := DecodeID(string(publicID))
	if err != nil {
		return nil, core.Errorw("cannot decode keys", err)
	}

	pr := eciesgo.NewPrivateKeyFromBytes(privateKey)
	if pr == nil {
		return nil, fmt.Errorf("cannot convert bytes to secp256k1 private key")
	}

	pu, err := eciesgo.NewPublicKeyFromBytes(publicKey)
	if err != nil {
		return nil, core.Errorw("cannot convert bytes to secp256k1 public key", err)
	}

	data, err := pr.ECDH(pu)
	if err != nil {
		return nil, core.Errorw("cannot perform ECDH", err)
	}

	h := sha256.Sum256(data)
	return h[:], nil
}
