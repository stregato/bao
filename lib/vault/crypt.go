package vault

import (
	"database/sql"
	"encoding/binary"
	"errors"
	"io"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
)

const (
	headerV1PrefixSize = 16
	headerV1Version    = 1

	encMethodPublic = 0
	encMethodAES    = 1
	encMethodEC     = 2
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
		return nil, core.Error(core.FileError, "cannot sign file head in encodeHead", err)
	}
	data := append(sign, buf...)
	core.End("file %s, shortID %d, data length %d", file.Name, shortID, len(data))
	return data, nil
}

func isNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, sqlx.ErrNoRows)
}

func decodeFile(data []byte, myShortID uint64, getUserId func(shortID uint64) (userID security.PublicID, err error)) (File, bool, error) {
	core.Start("data length %d", len(data))
	if len(data) < 74 {
		return File{}, false, core.Error(core.GenericError, "invalid data length: %d", len(data))
	}

	var file File
	var err error

	sign := data[:64]
	data = data[64:]
	if len(data) < 34 {
		return File{}, false, core.Error(core.GenericError, "invalid data length: %d", len(data))
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
		return File{}, false, core.Error(core.GenericError, "invalid data length: %d", len(data))
	}

	file.Name = string(data[34 : 34+nameLen])
	if attrsLen > 0 {
		file.Attrs = make([]byte, attrsLen)
		copy(file.Attrs, data[34+nameLen:34+nameLen+attrsLen])
	}

	userID, err := getUserId(shortID)
	if err != nil {
		if isNotFound(err) && shortID != myShortID {
			core.Info("unknown author short ID %d for file %s, skipping as not-for-me", shortID, file.Name)
			return File{}, true, nil
		}
		return File{}, false, core.Error(core.DbError, "cannot get user ID from short ID %d", shortID, err)
	}
	file.AuthorId = userID
	if !security.Verify(file.AuthorId, data, sign) {
		return File{}, false, core.Error(core.FileError, "signature verification failed for file head", err)
	}

	core.End("file %s", file.Name)
	return file, false, nil
}

// encodeFileHead encrypts the file head with the given key, including the name, size, and modification time.
// First, it copies the content to a binary buffer, then encrypts the buffer with the given key.
func encodeHead(encMethod string, file File, ecRecipient security.PublicID, authorPrivateID security.PrivateID,
	getKey func(keyId uint64) (key security.AESKey, err error)) ([]byte, error) {
	core.Start("file name %s, keyId %d", file.Name, file.KeyId)

	data, err := encodeFile(file, authorPrivateID)

	var method byte
	var ref uint64
	switch encMethod {
	case "public":
		method = encMethodPublic
	case "ec":
		userID := ecRecipient
		if userID == "" {
			return nil, core.Error(core.ParseError, "missing ec recipient for file %s", file.Name)
		}
		data, err = security.EcEncrypt(security.PublicID(userID), data)
		if err != nil {
			return nil, core.Error(core.GenericError, "invalid public ID %s", userID, err)
		}
		method = encMethodEC
		ref = userID.Hash()
	default: // aes
		key, err := getKey(file.KeyId)
		if err != nil {
			return nil, core.Error(core.DbError, "cannot get key for key id %d in encodeHead", file.KeyId, err)
		}
		if key == nil {
			core.End("no key found for key id %d", file.KeyId)
			return nil, nil // No key found for this file, it cannot be encrypted
		}
		method = encMethodAES
		ref = file.KeyId
		data, err = security.EncryptAES(data, key)
		if err != nil {
			return nil, core.Error(core.EncodeError, "cannot encrypt head in Bao.Write, name %v", file.Name, err)
		}
	}

	prefix := make([]byte, headerV1PrefixSize)
	prefix[0] = headerV1Version
	prefix[1] = method
	binary.LittleEndian.PutUint32(prefix[4:8], uint32(unixEpochSeconds(file.ExpiresAt)))
	binary.LittleEndian.PutUint64(prefix[8:], ref)

	core.End("successfully encoded file head for %s", file.Name)
	return append(prefix, data...), nil
}

// decodeFileHead decrypts the file head with the given key, including the name, size, modification time, and other metadata.
// First, it decrypts the data with the given key, then extracts the file size, modification time, and name.
func decodeHead(data []byte, userPrivateID security.PrivateID,
	getKey func(keyId uint64) (security.AESKey, error), getUserId func(shortID uint64) (security.PublicID, error)) (file File, notForMe bool, retryAfterBlockchain bool, err error) {
	core.Start("data length %d", len(data))
	if len(data) < headerV1PrefixSize+74 {
		return File{}, false, false, core.Error(core.GenericError, "invalid data length: %d", len(data))
	}

	version := data[0]
	if version != headerV1Version {
		return File{}, false, false, core.Error(core.ParseError, "unsupported header version: %d", version)
	}
	method := data[1]
	expiresAtSec := int64(binary.LittleEndian.Uint32(data[4:8]))
	ref := binary.LittleEndian.Uint64(data[8:16])
	data = data[headerV1PrefixSize:]
	file.ExpiresAt = timeFromEpochSeconds(expiresAtSec)
	userID, err := userPrivateID.PublicID()
	if err != nil {
		return File{}, false, false, core.Error(core.DbError, "cannot get public ID from private ID in decodeHead", err)
	}
	myShortID := userID.Hash()
	var decodedKeyID uint64
	var decodedRecipient security.PublicID
	var decodedFlagsMask Flags

	switch method {
	case encMethodPublic:
		decodedKeyID = 0
		decodedFlagsMask = 0
	case encMethodEC:
		if myShortID != ref { // only the intended user can decrypt with their private ID
			// Normal path: this file is addressed to another recipient.
			return file, true, false, nil
		}
		// Use user's private ID to decrypt
		data, err = security.EcDecrypt(userPrivateID, data)
		if err != nil {
			return File{}, false, false, core.Error(core.FileError, "cannot decrypt file head in decodeHead", err)
		}
		decodedKeyID = 0
		decodedRecipient = userID
		decodedFlagsMask = EcEncryption
	case encMethodAES:
		key, err := getKey(ref)
		if err != nil {
			if core.ErrorCode(err) == core.AccessDenied {
				// Normal path: this file is encrypted with a key the current user does not have.
				// Skip it without failing the whole sync.
				return file, true, false, nil
			}
			return File{}, false, false, core.Error(core.DbError, "cannot get key for key id %d in decodeHead", ref, err)
		}
		if key == nil {
			core.End("no key found for key id %d", ref)
			return File{}, false, false, nil // No key found for this file, it cannot be decrypted
		}
		data, err = security.DecryptAES(data, key)
		if err != nil {
			return File{}, false, false, core.Error(core.FileError, "cannot decrypt file head in decodeHead", err)
		}
		decodedKeyID = ref
		decodedFlagsMask = AESEncryption
	default:
		return File{}, false, false, core.Error(core.ParseError, "unsupported encryption method: %d", method)
	}

	var unknownAuthor bool
	decoded, unknownAuthor, err := decodeFile(data, myShortID, getUserId)
	if err != nil {
		return File{}, false, false, core.Error(core.FileError, "cannot decode file head in decodeHead", err)
	}
	if unknownAuthor {
		// Temporary condition: user table might be stale until blockchain access updates are imported.
		return File{}, false, true, nil
	}
	file = decoded
	file.ExpiresAt = timeFromEpochSeconds(expiresAtSec)
	file.KeyId = decodedKeyID
	file.EcRecipient = decodedRecipient
	file.Flags &^= AESEncryption | EcEncryption
	file.Flags |= decodedFlagsMask

	core.End("successfully decoded file head for %s", file.Name)
	return file, false, false, nil
}

func encryptReader(encMethod string, file File, ecRecipient security.PublicID, r io.ReadSeeker,
	getKey func(keyId uint64) (key security.AESKey, err error)) (io.ReadSeeker, error) {
	core.Start("file name %s, keyId %d", file.Name, file.KeyId)

	iv, err := getIv(file.Name)
	if err != nil {
		return nil, core.Error(core.DbError, "cannot get iv in encryptReader, name %v", file.Name, err)
	}

	switch encMethod {
	case "public":
		return r, nil // No encryption for public group
	case "ec":
		userID := ecRecipient
		if userID == "" {
			return nil, core.Error(core.ParseError, "missing ec recipient for file %s", file.Name)
		}
		r, err = security.EcEncryptReader(userID, r, iv)
		if err != nil {
			return nil, core.Error(core.FileError, "cannot encrypt reader for file %s", file.Name, err)
		}
		core.End("successfully created elliptic encrypted reader for file %s", file.Name)
		return r, nil
	default: // aes
		key, err := getKey(file.KeyId)
		if err != nil {
			return nil, core.Error(core.DbError, "cannot get key for key id %d in encryptReader", file.KeyId, err)
		}
		r, err = security.EncryptReader(r, key, iv)
		if err != nil {
			return nil, core.Error(core.FileError, "cannot encrypt reader for file %s", file.Name, err)
		}
		core.End("successfully created symmetric encrypted reader for file %s", file.Name)
		return r, nil
	}
}

func decryptWriter(encMethod string, privateID security.PrivateID, file File, f io.Writer,
	getKey func(keyId uint64) (key security.AESKey, err error)) (io.Writer, error) {
	iv, err := getIv(file.Name)
	if err != nil {
		return nil, core.Error(core.DbError, "cannot get iv for file %s", file.Name, err)
	}

	var w io.Writer
	switch encMethod {
	case "public":
		w = f // No encryption, write directly to the file
	case "ec":
		w, err = security.EcDecryptWriter(privateID, f, iv)
		if err != nil {
			return nil, core.Error(core.EncodeError, "cannot create ec decrypt writer for %s", file.Name, err)
		}
	default: // aes
		key, err := getKey(file.KeyId)
		if err != nil {
			return nil, core.Error(core.DbError, "cannot get key for file %s", file.Name, err)
		}
		if key == nil {
			return nil, core.Error(core.AccessDenied, "no key found for id %d", file.KeyId)
		}

		w, err = security.DecryptWriter(f, key, iv)
		if err != nil {
			return nil, core.Error(core.EncodeError, "cannot create decrypt writer for %s", file.Name, err)
		}
	}
	core.End("successfully created decrypt writer for file %s", file.Name)
	return w, nil
}
