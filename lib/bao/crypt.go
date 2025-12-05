package bao

import (
	"encoding/binary"
	"io"
	"time"

	"github.com/dgryski/go-farm"
	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
)

// encodeFileHead encrypts the file head with the given key, including the name, size, and modification time.
// First, it copies the content to a binary buffer, then encrypts the buffer with the given key.
func encodeHead(file File, id security.PrivateID, getKey func(keyId uint64) (key security.AESKey, err error),
	getScope func(keyId uint64) (group Group, err error)) ([]byte, error) {
	core.Start("file name %s, keyId %d", file.Name, file.KeyId)

	// name string, group Scope, size int64, modTime time.Time, flags uint32,
	// attrs []byte, id security.PrivateID, getKey func(group Scope) (uint64, []byte, error)) (head []byte, err error) {
	buf := make([]byte, 26)
	binary.LittleEndian.PutUint64(buf[:8], uint64(file.Size))
	binary.LittleEndian.PutUint64(buf[8:], uint64(file.ModTime.UnixMilli()))
	binary.LittleEndian.PutUint32(buf[16:], uint32(file.Flags))
	binary.LittleEndian.PutUint16(buf[20:], uint16(len(file.Name)))
	binary.LittleEndian.PutUint32(buf[22:], uint32(len(file.Attrs)))

	nameBytes := []byte(file.Name)
	buf = append(buf, nameBytes...)
	buf = append(buf, file.Attrs...)

	pid := id.PublicIDMust()
	buf = append(buf, pid.Bytes()...)
	// Append the file name to the buffer

	sign, err := security.Sign(id, buf)
	if err != nil {
		return nil, core.Errorw("cannot sign file head in encodeHead", err)
	}
	// iv, err := getIv(name)
	// if err != nil {
	// 	return nil, nil, core.Errorw(err, "cannot get iv in Bao.Write, name %v, group %v: %v", name, group)
	// }

	data := append(sign, buf...)

	switch {
	case file.KeyId == 0:
	case file.KeyId&(1<<63) != 0:
		group, err := getScope(file.KeyId)
		if err != nil {
			return nil, core.Errorw("cannot get group for key %d in encodeHead", file.KeyId, err)
		}
		data, err = security.EcEncrypt(security.PublicID(group), data)
		if err != nil {
			return nil, core.Errorw("cannot encrypt file head in encodeHead", err)
		}
	default:
		key, err := getKey(file.KeyId)
		if err != nil {
			return nil, core.Errorw("cannot get key for key id %d in encodeHead", file.KeyId, err)
		}
		if key == nil {
			core.End("no key found for key id %d", file.KeyId)
			return nil, nil // No key found for this file, it cannot be encrypted
		}
		data, err = security.EncryptAES(data, key)
		if err != nil {
			return nil, core.Errorw("cannot encrypt head in Bao.Write, name %v", file.Name, err)
		}

		// if size > 0 {
		// 	er, err = security.EncryptReader(r, key, iv)
		// 	if err != nil {
		// 		return nil, nil, core.Errorw(err, "cannot encrypt reader in Bao.Write, name %v, group %v: %v", name, group)
		// 	}
		// }
		//binary.LittleEndian.PutUint16(plain[:2], 2)
	}

	plain := make([]byte, 8)
	binary.LittleEndian.PutUint64(plain, file.KeyId)

	core.End("successfully encoded file head for %s", file.Name)
	return append(plain, data...), nil
}

// decodeFileHead decrypts the file head with the given key, including the name, size, and modification time.
// First, it decrypts the data with the given key, then extracts the file size, modification time, and name.
func decodeHead(data []byte, id security.PrivateID, getKey func(keyId uint64) (security.AESKey, error)) (File, error) {
	core.Start("data length %d", len(data))
	if len(data) < 74 {
		return File{}, core.Errorw("invalid data length: %d", len(data))
	}

	var file File

	file.KeyId = binary.LittleEndian.Uint64(data[0:8])
	data = data[8:]
	switch {
	case file.KeyId == 0:
	case file.KeyId&(1<<63) != 0:
		// For public groups, we use the public ID to decrypt the data
		publicId, err := id.PublicID()
		if err != nil {
			return File{}, core.Errorw("provided private ID is not valid in decodeHead", err)
		}
		hash := farm.Hash64([]byte(publicId)) | (1 << 63)
		if file.KeyId != hash {
			return File{}, nil
		}

		data, err = security.EcDecrypt(id, data)
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

	sign := data[:64]
	data = data[64:]
	if len(data) < 22+security.PublicIDLenght {
		return File{}, core.Errorw("invalid data length: %d", len(data))
	}

	file.Size = int64(binary.LittleEndian.Uint64(data[:8]))
	file.AllocatedSize = file.Size
	timeInMs := int64(binary.LittleEndian.Uint64(data[8:16]))
	file.ModTime = time.UnixMilli(timeInMs)
	file.Flags = Flags(binary.LittleEndian.Uint32(data[16:20]))

	nameLen := int(binary.LittleEndian.Uint16(data[20:22]))
	attrsLen := int(binary.LittleEndian.Uint32(data[22:26]))

	if len(data) < 26+nameLen+attrsLen+security.PublicIDLenght {
		return File{}, core.Errorw("invalid data length: %d", len(data))
	}

	file.Name = string(data[26 : 26+nameLen])
	if attrsLen > 0 {
		file.Attrs = make([]byte, attrsLen)
		copy(file.Attrs, data[26+nameLen:26+nameLen+attrsLen])
	}

	var err error
	file.AuthorId, err = security.PublicIDFromBytes(data[26+nameLen+attrsLen : 26+nameLen+attrsLen+security.PublicIDLenght])
	if err != nil {
		return File{}, core.Errorw("cannot get public id from bytes", err)
	}
	if !security.Verify(file.AuthorId, data, sign) {
		return File{}, core.Errorw("signature verification failed for file head", err)
	}

	core.End("successfully decoded file head for %s", file.Name)
	return file, nil
}

func encryptReader(file File, r io.ReadSeeker,
	getKey func(keyId uint64) (key security.AESKey, err error),
	getScope func(keyId uint64) (group Group, err error)) (io.ReadSeeker, error) {
	core.Start("file name %s, keyId %d", file.Name, file.KeyId)

	iv, err := getIv(file.Name)
	if err != nil {
		return nil, core.Errorw("cannot get iv in encryptReader, name %v", file.Name, err)
	}

	switch {
	case file.KeyId == 0:
		return r, nil // No encryption for public group
	case file.KeyId&(1<<63) != 0:
		group, err := getScope(file.KeyId)
		if err != nil {
			return nil, core.Errorw("cannot get group for key %d in encryptReader", file.KeyId, err)
		}
		r, err = security.EcEncryptReader(security.PublicID(group), r, iv)
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
