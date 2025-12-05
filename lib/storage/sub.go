package storage

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
)

type sub struct {
	Store          Store
	Base           string
	PropagateClose bool
}

func Sub(s Store, base string, propagateClose bool) Store {
	s2, isSub := s.(*sub)
	if isSub {
		return &sub{s2.Store, path.Join(s2.Base, base), propagateClose}
	} else {
		return &sub{s, base, propagateClose}
	}
}

func (s *sub) ID() string {
	return s.Store.ID()
}

func (s *sub) ReadDir(name string, filter Filter) ([]fs.FileInfo, error) {
	return s.Store.ReadDir(path.Join(s.Base, name), filter)
}

// Read reads data from a file into a writer
func (s *sub) Read(name string, rang *Range, dest io.Writer, progress chan int64) error {
	return s.Store.Read(path.Join(s.Base, name), rang, dest, progress)
}

// Write writes data to a file name. An existing file is overwritten
func (s *sub) Write(name string, source io.ReadSeeker, progress chan int64) error {
	return s.Store.Write(path.Join(s.Base, name), source, progress)
}

// Stat provides statistics about a file
func (s *sub) Stat(name string) (os.FileInfo, error) {
	return s.Store.Stat(path.Join(s.Base, name))
}

// Delete deletes a file
func (s *sub) Delete(name string) error {
	return s.Store.Delete(path.Join(s.Base, name))
}

// Close closes the store
func (s *sub) Close() error {
	if s.PropagateClose {
		return s.Store.Close()
	}
	return nil
}

// func (s *sub) LastChange(dir string) (time.Time, error) {
// 	return s.Store.LastChange(path.Join(s.Base, dir))
// }

// String returns a human-readable representation of the storer (e.g. sftp://user@host/path)
func (s *sub) String() string {
	return fmt.Sprintf("%s/%s", s.Store, s.Base)
}

func (s *sub) Describe() Description {
	return s.Store.Describe()
}
