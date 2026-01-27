//go:build js

package store

import (
	"errors"
	"io"
	"io/fs"
	"os"
)

// Minimal js stub to satisfy interface; returns unsupported errors.

type Azure struct{}

func OpenAzure(connectionUrl string) (Store, error) {
	return nil, errors.New("azure store.not supported in wasm/js build")
}
func (a *Azure) ID() string { return "azure://unsupported" }
func (a *Azure) Read(string, *Range, io.Writer, chan int64) error {
	return errors.New("azure store.unsupported")
}
func (a *Azure) Write(string, io.ReadSeeker, chan int64) error {
	return errors.New("azure store.unsupported")
}
func (a *Azure) ReadDir(string, Filter) ([]fs.FileInfo, error) {
	return nil, errors.New("azure store.unsupported")
}
func (a *Azure) Stat(string) (os.FileInfo, error) {
	return nil, errors.New("azure store.unsupported")
}
func (a *Azure) Delete(string) error   { return errors.New("azure store.unsupported") }
func (a *Azure) Close() error          { return nil }
func (a *Azure) String() string        { return a.ID() }
func (a *Azure) Describe() Description { return Description{} }
