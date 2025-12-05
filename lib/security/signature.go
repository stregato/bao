package security

import (
	"bytes"
	"crypto/ed25519"

	"github.com/stregato/bao/lib/core"
)

type PublicKey ed25519.PublicKey
type PrivateKey ed25519.PrivateKey

const (
	PublicKeySize  = ed25519.PublicKeySize
	PrivateKeySize = ed25519.PrivateKeySize
	SignatureSize  = ed25519.SignatureSize
)

type SignedData struct {
	Signature [SignatureSize]byte
	Signer    PublicKey
}

type Public struct {
	Id    PublicKey
	Nick  string
	Email string
}

func Sign(privateID PrivateID, data []byte) ([]byte, error) {
	_, signKey, err := DecodeID(string(privateID))
	if core.IsErr(err, "cannot decode keys: %v") {
		return nil, err
	}

	return ed25519.Sign(ed25519.NewKeyFromSeed(signKey), data), nil
}

func Verify(publicID PublicID, data []byte, sig []byte) bool {
	_, signKey, err := DecodeID(string(publicID))
	if core.IsErr(err, "cannot decode keys: %v") {
		return false
	}

	for off := 0; off < len(sig); off += SignatureSize {
		if func() bool {
			defer func() { recover() }()
			return ed25519.Verify(ed25519.PublicKey(signKey), data, sig[off:off+SignatureSize])
		}() {
			return true
		}
	}
	return false
}

type SignedHashEvidence struct {
	Key       []byte `json:"k"`
	Signature []byte `json:"s"`
}

type SignedHash struct {
	Hash       []byte
	Signatures map[PublicID][]byte
}

func NewSignedHash(hash []byte, i PrivateID) (SignedHash, error) {
	publicID := i.PublicIDMust()
	signature, err := Sign(i, hash)
	if core.IsErr(err, "cannot sign with identity %s: %v", publicID) {
		return SignedHash{}, err
	}

	return SignedHash{
		Hash:       hash,
		Signatures: map[PublicID][]byte{publicID: signature},
	}, nil
}

func AppendToSignedHash(s SignedHash, privateID PrivateID) error {
	publicID := privateID.PublicIDMust()
	signature, err := Sign(privateID, s.Hash)
	if core.IsErr(err, "cannot sign with identity %s: %v", publicID) {
		return err
	}
	s.Signatures[publicID] = signature
	return nil
}

func VerifySignedHash(s SignedHash, trusts []PrivateID, hash []byte) bool {
	if !bytes.Equal(s.Hash, hash) {
		return false
	}

	for _, trust := range trusts {
		id := trust.PublicIDMust()
		if signature, ok := s.Signatures[id]; ok {
			if Verify(id, hash, signature) {
				return true
			}
		}
	}
	return false
}
