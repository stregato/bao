package store

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
)

type encrypted struct {
	Store          Store
	Key            []byte
	Nonce          []byte
	PropagateClose bool
}

func EncryptNames(s Store, key []byte, nonce []byte, propagateClose bool) Store {
	return &encrypted{s, key, nonce, propagateClose}
}

func (s *encrypted) ID() string {
	return s.Store.ID()
}

func (s *encrypted) ReadDir(name string, filter Filter) ([]fs.FileInfo, error) {
	c, err := security.AESEncrypt(s.Key, s.Nonce, []byte(name))
	if err != nil {
		return nil, err
	}
	name = base64.StdEncoding.EncodeToString(c)
	ls, err := s.Store.ReadDir(name, filter)
	if err != nil {
		return nil, err
	}

	var files []fs.FileInfo
	for _, l := range ls {
		data, err := base64.StdEncoding.DecodeString(l.Name())
		if core.IsWarn(err, "cannot decode %s: %v", l.Name()) {
			continue
		}
		d, err := security.AESDecrypt(s.Key, s.Nonce, data)
		if core.IsWarn(err, "cannot decrypt %s: %v", l.Name()) {
			continue
		}
		files = append(files, simpleFileInfo{
			name:    string(d),
			size:    l.Size(),
			modTime: l.ModTime(),
			isDir:   l.IsDir(),
		})
	}
	return files, nil
}

// Read reads data from a file into a writer
func (s *encrypted) Read(name string, rang *Range, dest io.Writer, progress chan int64) error {
	c, err := security.AESEncrypt(s.Key, s.Nonce, []byte(name))
	if err != nil {
		return err
	}
	name = base64.StdEncoding.EncodeToString(c)
	return s.Store.Read(name, rang, dest, progress)
}

// Write writes data to a file name. An existing file is overwritten
func (s *encrypted) Write(name string, source io.ReadSeeker, progress chan int64) error {
	c, err := security.AESEncrypt(s.Key, s.Nonce, []byte(name))
	if err != nil {
		return err
	}
	name = base64.StdEncoding.EncodeToString(c)
	return s.Store.Write(name, source, progress)
}

// Stat provides statistics about a file
func (s *encrypted) Stat(name string) (os.FileInfo, error) {
	c, err := security.AESEncrypt(s.Key, s.Nonce, []byte(name))
	if err != nil {
		return nil, err
	}
	name = base64.StdEncoding.EncodeToString(c)

	return s.Store.Stat(name)
}

// Delete deletes a file
func (s *encrypted) Delete(name string) error {
	c, err := security.AESEncrypt(s.Key, s.Nonce, []byte(name))
	if err != nil {
		return err
	}
	name = base64.StdEncoding.EncodeToString(c)

	return s.Store.Delete(name)
}

// Close closes the store
func (s *encrypted) Close() error {
	if s.PropagateClose {
		return s.Store.Close()
	}
	return nil
}

// func (s *encrypted) LastChange(dir string) (time.Time, error) {
// 	return s.Store.LastChange(dir)
// }

func (s *encrypted) Describe() Description {
	return s.Store.Describe()
}

// String returns a human-readable representation of the storer (e.g. sftp://user@host/path)
func (s *encrypted) String() string {
	return fmt.Sprintf("%s,enc", s.Store)
}
