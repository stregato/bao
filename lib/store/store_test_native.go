//go:build !js

package store

import (
	"os"
	"path"

	"github.com/stregato/bao/lib/core"
	"gopkg.in/yaml.v2"
)

// LoadTestURLs loads test URLs from credentials file (non-JS platforms only)
func LoadTestURLs() (urls map[string]StoreConfig) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(core.Error(core.DbError, "cannot get user home dir", err))
	}
	filename := path.Join(homeDir, "credentials.yaml")
	_, err = os.Stat(filename)
	if err != nil {
		filename = "../credentials.yaml"
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		panic(core.Error(core.FileError, "cannot read file %s", filename, err))
	}

	err = yaml.Unmarshal(data, &urls)
	if err != nil {
		panic(core.Error(core.ParseError, "cannot parse file %s", filename, err))
	}
	return urls
}

// NewTestStore creates a new test store (non-JS platforms only)
func NewTestStore(id string) Store {
	c := LoadTestConfig(nil, id)
	store, err := Open(c)
	if err != nil {
		panic(core.Error(core.GenericError, "cannot open syestore %s", id, err))
	}
	ls, _ := store.ReadDir("", Filter{})
	for _, l := range ls {
		store.Delete(l.Name())
	}

	return store
}
