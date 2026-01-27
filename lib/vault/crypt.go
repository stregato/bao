package vault

import (
	"encoding/binary"
	"io"
	"strings"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
)

func encodeFile(file File, authorPrivateID security.PrivateID) ([]byte, error) {
	core.Start("file name %s", file.Name)
	// name string, group Scope, size int64, modTime time.Time, flags uint32,
	// attrs []byte, id security.PrivateID, getKey func(group Scope) (uint64, []byte, error)) (head []byte, err error) {

	authorID := authorPrivateID.PublicIDMust()
	shortID := authorID.Hash()

	buf := make([]byte, 34)
	binary.LittleEndian.PutUint64(buf[:8], uint64(file.Size))
	binary.LittleEndian.PutUint64(buf[8:], uint64(file.ModTime.UnixMilli()))
	binary.LittleEndian.PutUint32(buf[16:], uint32(file.Flags))
	binary.LittleEndian.PutUint16(buf[20:], uint16(len(file.Name)))
	binary.LittleEndian.PutUint32(buf[22:], uint32(len(file.Attrs)))
	binary.LittleEndian.PutUint64(buf[26:], shortID)

	nameBytes := []byte(file.Name)
	buf = append(buf, nameBytes...)
	buf = append(buf, file.Attrs...)

	sign, err := security.Sign(authorPrivateID, buf)
	if err != nil {
		return nil, core.Errorw("cannot sign file head in encodeHead", err)
	}
	data := append(sign, buf...)
	return data, nil
}

func decodeFile(data []byte, getUserId func(shortID uint64) (userID security.PublicID, err error)) (File, error) {
	core.Start("data length %d", len(data))
	if len(data) < 74 {
		return File{}, core.Errorw("invalid data length: %d", len(data))
	}

	var file File
	var err error

	sign := data[:64]
	data = data[64:]
	if len(data) < 34 {
		return File{}, core.Errorw("invalid data length: %d", len(data))
	}

	file.Size = int64(binary.LittleEndian.Uint64(data[:8]))
	file.AllocatedSize = file.Size
	timeInMs := int64(binary.LittleEndian.Uint64(data[8:16]))
	file.ModTime = time.UnixMilli(timeInMs)
	file.Flags = Flags(binary.LittleEndian.Uint32(data[16:20]))

	nameLen := int(binary.LittleEndian.Uint16(data[20:22]))
	attrsLen := int(binary.LittleEndian.Uint32(data[22:26]))
	shortID := binary.LittleEndian.Uint64(data[26:34])

	if len(data) < 34+nameLen+attrsLen {
		return File{}, core.Errorw("invalid data length: %d", len(data))
	}

	file.Name = string(data[34 : 34+nameLen])
	if attrsLen > 0 {
		file.Attrs = make([]byte, attrsLen)
		copy(file.Attrs, data[34+nameLen:34+nameLen+attrsLen])
	}

	userID, err := getUserId(shortID)
	if err != nil {
		return File{}, core.Errorw("cannot get user ID from short ID %d", shortID, err)
	}
	file.AuthorId = userID
	if !security.Verify(file.AuthorId, data, sign) {
		return File{}, core.Errorw("signature verification failed for file head", err)
	}

	core.End("successfully decoded file head for %s", file.Name)
	return file, nil
}

// encodeFileHead encrypts the file head with the given key, including the name, size, and modification time.
// First, it copies the content to a binary buffer, then encrypts the buffer with the given key.
func encodeHead(realm Realm, file File, authorPrivateID security.PrivateID,
	getKey func(keyId uint64) (key security.AESKey, err error)) ([]byte, error) {
	core.Start("file name %s, keyId %d", file.Name, file.KeyId)

	data, err := encodeFile(file, authorPrivateID)

	var prefix uint64
	switch realm {
	case All:
	case Home:
		firstDir := strings.SplitN(string(file.Name), "/", 2)[0]
		userID := security.PublicID(firstDir)
		data, err = security.EcEncrypt(security.PublicID(userID), data)
		if err != nil {
			return nil, core.Errorw("invalid public ID %s", userID, err)
		}
		prefix = userID.Hash()
	default:
		key, err := getKey(file.KeyId)
		if err != nil {
			return nil, core.Errorw("cannot get key for key id %d in encodeHead", file.KeyId, err)
		}
		if key == nil {
			core.End("no key found for key id %d", file.KeyId)
			return nil, nil // No key found for this file, it cannot be encrypted
		}
		prefix = file.KeyId
		data, err = security.EncryptAES(data, key)
		if err != nil {
			return nil, core.Errorw("cannot encrypt head in Bao.Write, name %v", file.Name, err)
		}
	}

	plain := make([]byte, 8)
	binary.LittleEndian.PutUint64(plain, prefix)

	core.End("successfully encoded file head for %s", file.Name)
	return append(plain, data...), nil
}

// decodeFileHead decrypts the file head with the given key, including the name, size, modification time, and other metadata.
// First, it decrypts the data with the given key, then extracts the file size, modification time, and name.
func decodeHead(realm Realm, data []byte, userPrivateID security.PrivateID,
	getKey func(keyId uint64) (security.AESKey, error), getUserId func(shortID uint64) (security.PublicID, error)) (File, error) {
	core.Start("data length %d", len(data))
	if len(data) < 74 {
		return File{}, core.Errorw("invalid data length: %d", len(data))
	}

	var file File
	var err error

	file.KeyId = binary.LittleEndian.Uint64(data[0:8])
	data = data[8:]
	switch realm {
	case All:
	case Home:
		userID, err := userPrivateID.PublicID()
		if err != nil {
			return File{}, core.Errorw("cannot get public ID from private ID in decodeHead", err)
		}
		shortID := userID.Hash()
		if shortID != file.KeyId { // only the home user can decrypt with their private ID
			return File{}, core.Errorw("short ID mismatch: expected %d, got %d", shortID, file.KeyId)
		}
		// Use user's private ID to decrypt
		data, err = security.EcDecrypt(userPrivateID, data)
		if err != nil {
			return File{}, core.Errorw("cannot decrypt file head in decodeHead", err)
		}
	default:
		key, err := getKey(file.KeyId)
		if err != nil {
			return File{}, core.Errorw("cannot get key for key id %d in decodeHead", file.KeyId, err)
		}
		if key == nil {
			core.End("no key found for key id %d", file.KeyId)
			return File{}, nil // No key found for this file, it cannot be decrypted
		}
		data, err = security.DecryptAES(data, key)
		if err != nil {
			return File{}, core.Errorw("cannot decrypt file head in decodeHead", err)
		}
	}

	file, err = decodeFile(data, getUserId)
	if err != nil {
		return File{}, core.Errorw("cannot decode file head in decodeHead", err)
	}

	core.End("successfully decoded file head for %s", file.Name)
	return file, nil
}

func encryptReader(realm Realm, file File, r io.ReadSeeker,
	getKey func(keyId uint64) (key security.AESKey, err error)) (io.ReadSeeker, error) {
	core.Start("file name %s, keyId %d", file.Name, file.KeyId)

	iv, err := getIv(file.Name)
	if err != nil {
		return nil, core.Errorw("cannot get iv in encryptReader, name %v", file.Name, err)
	}

	switch realm {
	case All:
		return r, nil // No encryption for public group
	case Home:
		firstDir := strings.SplitN(string(file.Name), "/", 2)[0]
		userID := security.PublicID(firstDir)
		r, err = security.EcEncryptReader(userID, r, iv)
		if err != nil {
			return nil, core.Errorw("cannot encrypt reader for file %s", file.Name, err)
		}
		core.End("successfully created elliptic encrypted reader for file %s", file.Name)
		return r, nil
	default:
		key, err := getKey(file.KeyId)
		if err != nil {
			return nil, core.Errorw("cannot get key for key id %d in encryptReader", file.KeyId, err)
		}
		r, err = security.EncryptReader(r, key, iv)
		if err != nil {
			return nil, core.Errorw("cannot encrypt reader for file %s", file.Name, err)
		}
		core.End("successfully created symmetric encrypted reader for file %s", file.Name)
		return r, nil
	}
}

func decryptWriter(realm Realm, privateID security.PrivateID, file File, f io.Writer,
	getKey func(keyId uint64) (key security.AESKey, err error)) (io.Writer, error) {
	iv, err := getIv(file.Name)
	if err != nil {
		return nil, core.Errorw("cannot get iv for file %s", file.Name, err)
	}

	var w io.Writer
	switch realm {
	case All:
		w = f // No encryption, write directly to the file
	case Home: // EC encryption
		w, err = security.EcDecryptWriter(privateID, f, iv)
		if err != nil {
			return nil, core.Errorw("cannot create ec decrypt writer for %s", file.Name, err)
		}
	default: // AES encryption
		key, err := getKey(file.KeyId)
		if err != nil {
			return nil, core.Errorw("cannot get key for file %s", file.Name, err)
		}

		w, err = security.DecryptWriter(f, key, iv)
		if err != nil {
			return nil, core.Errorw("cannot create decrypt writer for %s", file.Name, err)
		}
	}
	core.End("successfully created decrypt writer for file %s", file.Name)
	return w, nil
}
