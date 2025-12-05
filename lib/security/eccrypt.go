package security

import (
	"io"

	eciesgo "github.com/ecies/go/v2"
	"github.com/stregato/bao/lib/core"
)

func EcEncrypt(publicID PublicID, data []byte) ([]byte, error) {
	cryptKey, _, err := DecodeID(publicID.String())
	if err != nil {
		return nil, core.Errorw("cannot decode keys", err)
	}

	pk, err := eciesgo.NewPublicKeyFromBytes(cryptKey)
	if err != nil {
		return nil, core.Errorw("cannot convert bytes to secp256k1 public key", err)
	}
	data, err = eciesgo.Encrypt(pk, data)
	if err != nil {
		return nil, core.Errorw("cannot encrypt with secp256k1", err)
	}
	return data, err
}

func EcDecrypt(privateID PrivateID, data []byte) ([]byte, error) {
	cryptKey, _, err := DecodeID(privateID.String())
	if core.IsWarn(err, "cannot decode keys: %v") {
		return nil, err
	}

	data, err = eciesgo.Decrypt(eciesgo.NewPrivateKeyFromBytes(cryptKey), data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

type EcEncryptingReadSeeker struct {
	pos int64
	key []byte
	r   io.ReadSeeker
}

func EcEncryptReader(publicID PublicID, r io.ReadSeeker, iv []byte) (io.ReadSeeker, error) {
	key := core.GenerateRandomBytes(32)
	r, err := EncryptReader(r, key, iv)
	if err != nil {
		return nil, err
	}
	key, err = EcEncrypt(publicID, key)
	if err != nil {
		return nil, err
	}

	return &EcEncryptingReadSeeker{
		key: key,
		r:   r,
	}, nil
}

func (r *EcEncryptingReadSeeker) Read(p []byte) (n int, err error) {
	headLeft := len(r.key) - int(r.pos)
	if headLeft > 0 {
		n = copy(p, r.key[r.pos:])
		r.pos += int64(n)
	}

	n2, err := r.r.Read(p[n:])
	return n + n2, err
}

func (er *EcEncryptingReadSeeker) Seek(offset int64, whence int) (int64, error) {
	return er.r.Seek(offset, whence)
}

type EcDecryptingWriter struct {
	w         io.Writer
	dw        io.Writer
	key       []byte
	iv        []byte
	privateID PrivateID
}

func EcDecryptWriter(privateID PrivateID, w io.Writer, iv []byte) (io.Writer, error) {
	return &EcDecryptingWriter{
		w:         w,
		iv:        iv,
		privateID: privateID,
	}, nil
}

func (w *EcDecryptingWriter) Write(p []byte) (n int, err error) {
	if w.dw != nil {
		return w.dw.Write(p)
	}
	if len(w.key) < 129 {
		n = min(129-len(w.key), len(p))
		w.key = append(w.key, p[:n]...)
		p = p[n:]
	}
	if len(w.key) == 129 {
		w.key, err = EcDecrypt(w.privateID, w.key)
		if err != nil {
			return n, err
		}
		w.dw, err = DecryptWriter(w.w, w.key, w.iv)
		if err != nil {
			return n, err
		}
	}
	if w.dw != nil && len(p) > 0 {
		n2, err := w.dw.Write(p)
		return n + n2, err
	}
	return n, nil
}
