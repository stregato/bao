package security

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	eciesgo "github.com/ecies/go/v2"
	"github.com/stregato/bao/lib/core"
)

var ErrInvalidSignature = errors.New("signature is invalid")
var ErrInvalidID = errors.New("ID is neither a public or private key")

const (
	Secp256k1               = "secp256k1"
	secp256k1PublicKeySize  = 33
	secp256k1PrivateKeySize = 32

	Ed25519 = "ed25519"
)

type Key struct {
	Public  []byte `json:"pu"`
	Private []byte `json:"pr,omitempty"`
}

type PublicID string
type PrivateID string

const PublicIDLenght = 65
const PrivateIDLenght = 64

func NewPrivateID() (PrivateID, error) {
	core.Start("generating new private ID")
	privateCrypt, err := eciesgo.GenerateKey()
	if core.IsErr(err, "cannot generate secp256k1 key: %v") {
		return "", err
	}

	_, signKey, err := ed25519.GenerateKey(rand.Reader)
	if core.IsErr(err, "cannot generate ed25519 key: %v") {
		return "", err
	}
	privateSign := signKey[:ed25519.PrivateKeySize-ed25519.PublicKeySize]
	id := PrivateID(base64.URLEncoding.EncodeToString(append(privateCrypt.Bytes(), privateSign...)))
	core.End("")
	return id, nil
}

func NewPrivateIDMust() PrivateID {
	privateID, err := NewPrivateID()
	if err != nil {
		panic(err)
	}
	return privateID
}

func PrivateIDFromBytes(data []byte) (PrivateID, error) {
	if len(data) != secp256k1PrivateKeySize+ed25519.PrivateKeySize-ed25519.PublicKeySize {
		return "", fmt.Errorf("invalid private ID length")
	}
	return PrivateID(base64.URLEncoding.EncodeToString(data)), nil
}

func PublicIDFromBytes(data []byte) (PublicID, error) {
	if len(data) != secp256k1PublicKeySize+ed25519.PublicKeySize {
		return "", fmt.Errorf("invalid public ID length")
	}
	return PublicID(base64.URLEncoding.EncodeToString(data)), nil
}

func (privateID PrivateID) PublicID() (PublicID, error) {
	cryptKey, privateSign, err := DecodeID(string(privateID))
	if core.IsErr(err, "cannot decode private ID: %v") {
		panic(err)
	}

	privateCrypt := eciesgo.NewPrivateKeyFromBytes(cryptKey)
	publicSign := ed25519.NewKeyFromSeed(privateSign)[ed25519.PrivateKeySize-ed25519.PublicKeySize:]
	return PublicID(base64.URLEncoding.EncodeToString(
		append(privateCrypt.PublicKey.Bytes(true), publicSign...))), nil
}

func (privateID PrivateID) PublicIDMust() PublicID {
	publicID, err := privateID.PublicID()
	if err != nil {
		panic(err)
	}
	return publicID
}

func (publicID PublicID) String() string {
	return string(publicID)
}

func (privateID PrivateID) String() string {
	return string(privateID)
}

func (publicID PublicID) Bytes() []byte {
	data, err := base64.URLEncoding.DecodeString(publicID.String())
	if core.IsErr(err, "cannot decode base64: %v") {
		panic(err)
	}
	return data
}

func (privateID PrivateID) Bytes() []byte {
	data, err := base64.URLEncoding.DecodeString(privateID.String())
	if core.IsErr(err, "cannot decode base64: %v") {
		panic(err)
	}
	return data
}

func DecodeID(id string) (cryptKey []byte, signKey []byte, err error) {
	data, err := base64.URLEncoding.DecodeString(id)
	if core.IsErr(err, "cannot decode base64: %v") {
		return nil, nil, err
	}

	var split int
	if len(data) == secp256k1PrivateKeySize+ed25519.PrivateKeySize-ed25519.PublicKeySize {
		split = secp256k1PrivateKeySize
	} else if len(data) == secp256k1PublicKeySize+ed25519.PublicKeySize {
		split = secp256k1PublicKeySize
	} else {
		core.IsErr(ErrInvalidID, "invalid ID %s with length %d: %v", id, len(data))
		return nil, nil, ErrInvalidID
	}

	return data[:split], data[split:], nil
}
