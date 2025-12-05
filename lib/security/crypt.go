package security

import (
	"crypto/aes"
	"crypto/cipher"
	"io"
)

type EncryptingReadSeeker struct {
	inputSeeker io.ReadSeeker
	cipher      cipher.Stream
}

func (er *EncryptingReadSeeker) Read(p []byte) (n int, err error) {
	n, err = er.inputSeeker.Read(p)
	if n > 0 {
		er.cipher.XORKeyStream(p[:n], p[:n])
	}
	return n, err
}

func (er *EncryptingReadSeeker) Seek(offset int64, whence int) (int64, error) {
	return er.inputSeeker.Seek(offset, whence)
}

func EncryptReader(inputSeeker io.ReadSeeker, key []byte, iv []byte) (io.ReadSeeker, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	stream := cipher.NewCTR(block, iv)

	return &EncryptingReadSeeker{
		inputSeeker: inputSeeker,
		cipher:      stream,
	}, nil
}

type DecryptingWriter2 struct {
	outputWriter io.Writer
	cipher       cipher.Stream
}

func (dw *DecryptingWriter2) Write(p []byte) (n int, err error) {
	dw.cipher.XORKeyStream(p, p)
	return dw.outputWriter.Write(p)
}

func DecryptWriter(outputWriter io.Writer, key []byte, iv []byte) (io.Writer, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	stream := cipher.NewCTR(block, iv)

	return &DecryptingWriter2{
		outputWriter: outputWriter,
		cipher:       stream,
	}, nil
}
