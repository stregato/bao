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

func (p *PrivateID) Hash() uint64 {
	return core.SipHash(p.Bytes())
}

func (p *PublicID) Hash() uint64 {
	return core.SipHash(p.Bytes())
}

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

func NewKeyPair() (PublicID, PrivateID, error) {
	privateID, err := NewPrivateID()
	if err != nil {
		return "", "", err
	}
	publicID, err := privateID.PublicID()
	if err != nil {
		return "", "", err
	}
	return publicID, privateID, nil
}

func NewKeyPairMust() (PublicID, PrivateID) {
	publicID, privateID, err := NewKeyPair()
	if err != nil {
		panic(err)
	}
	return publicID, privateID
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
	cryptKey, privateSign, err := privateID.Decode()
	if core.IsErr(err, "cannot decode private ID: %v") {
		return "", err
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
func (publicID PublicID) Decode() (cryptKey []byte, signKey []byte, err error) {
	data, err := base64.URLEncoding.DecodeString(publicID.String())
	if core.IsErr(err, "cannot decode base64: %v") {
		return nil, nil, err
	}
	if len(data) != secp256k1PublicKeySize+ed25519.PublicKeySize {
		core.IsErr(ErrInvalidID, "invalid public ID %s with length %d", publicID, len(data))
		return nil, nil, ErrInvalidID
	}
	return data[:secp256k1PublicKeySize], data[secp256k1PublicKeySize:], nil
}

func (privateID PrivateID) Bytes() []byte {
	data, err := base64.URLEncoding.DecodeString(privateID.String())
	if core.IsErr(err, "cannot decode base64: %v") {
		panic(err)
	}
	return data
}

func (privateID PrivateID) Decode() (cryptKey []byte, signKey []byte, err error) {
	data, err := base64.URLEncoding.DecodeString(privateID.String())
	if core.IsErr(err, "cannot decode base64: %v") {
		return nil, nil, err
	}
	if len(data) != secp256k1PrivateKeySize+ed25519.PrivateKeySize-ed25519.PublicKeySize {
		core.IsErr(ErrInvalidID, "invalid private ID %s with length %d", privateID, len(data))
		return nil, nil, ErrInvalidID
	}
	return data[:secp256k1PrivateKeySize], data[secp256k1PrivateKeySize:], nil
}
