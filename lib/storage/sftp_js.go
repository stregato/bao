//go:build js

package storage

import (
	"errors"
	"io"
	"io/fs"
	"os"
)

// Minimal js stub for SFTP to satisfy interface; returns unsupported errors.

type SFTP struct{}

func OpenSFTP(connectionUrl string) (Store, error) {
	return nil, errors.New("sftp storage not supported in wasm/js build")
}
func (s *SFTP) ID() string { return "sftp://unsupported" }
func (s *SFTP) Read(string, *Range, io.Writer, chan int64) error {
	return errors.New("sftp storage unsupported")
}
func (s *SFTP) Write(string, io.ReadSeeker, chan int64) error {
	return errors.New("sftp storage unsupported")
}
func (s *SFTP) ReadDir(string, Filter) ([]fs.FileInfo, error) {
	return nil, errors.New("sftp storage unsupported")
}
func (s *SFTP) Stat(string) (os.FileInfo, error) { return nil, errors.New("sftp storage unsupported") }
func (s *SFTP) Rename(string, string) error      { return errors.New("sftp storage unsupported") }
func (s *SFTP) Delete(string) error              { return errors.New("sftp storage unsupported") }
func (s *SFTP) Close() error                     { return nil }
func (s *SFTP) String() string                   { return s.ID() }
func (s *SFTP) Describe() Description            { return Description{} }
