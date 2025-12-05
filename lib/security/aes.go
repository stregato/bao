package security

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

type AESKey []byte

// EncryptAES encrypts a byte array using AES encryption with the provided key.
func EncryptAES(data []byte, key AESKey) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Generate a random initialization vector (IV)
	iv := make([]byte, aes.BlockSize)
	_, err = io.ReadFull(rand.Reader, iv)
	if err != nil {
		return nil, err
	}

	// Create a new AES cipher block mode for CBC encryption
	mode := cipher.NewCBCEncrypter(block, iv)

	// Add padding to the data if needed
	paddedData := addPadding(data, aes.BlockSize)

	// Create a byte slice to hold the encrypted data
	encrypted := make([]byte, len(paddedData))

	// Encrypt the data
	mode.CryptBlocks(encrypted, paddedData)

	// Prepend the IV to the encrypted data
	encrypted = append(iv, encrypted...)

	return encrypted, nil
}

// DecryptAES decrypts an encrypted byte array using AES decryption with the provided key.
func DecryptAES(encrypted []byte, key AESKey) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(encrypted) == 0 || len(encrypted)%aes.BlockSize != 0 {
		return nil, errors.New("invalid data length")
	}

	// Split the encrypted data into IV and ciphertext
	iv := encrypted[:aes.BlockSize]
	ciphertext := encrypted[aes.BlockSize:]

	// Create a new AES cipher block mode for CBC decryption
	mode := cipher.NewCBCDecrypter(block, iv)

	// Decrypt the ciphertext
	decrypted := make([]byte, len(ciphertext))
	mode.CryptBlocks(decrypted, ciphertext)

	// Remove PKCS7 padding
	decrypted, err = removePadding(decrypted)
	if err != nil {
		return nil, err
	}

	return decrypted, nil
}

// addPadding adds PKCS7 padding to the data
func addPadding(data []byte, blockSize int) []byte {
	padding := blockSize - (len(data) % blockSize)
	paddedData := append(data, bytes.Repeat([]byte{byte(padding)}, padding)...)
	return paddedData
}

// removePadding removes PKCS7 padding from the data
func removePadding(data []byte) ([]byte, error) {
	padding := int(data[len(data)-1])
	if padding < 1 || padding > aes.BlockSize {
		return nil, errors.New("invalid padding")
	}
	if padding > len(data) {
		return nil, errors.New("invalid padding")
	}
	for i := len(data) - padding; i < len(data); i++ {
		if data[i] != byte(padding) {
			return nil, errors.New("invalid padding")
		}
	}
	return data[:len(data)-padding], nil
}
