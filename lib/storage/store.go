package storage

import (
	"io"
	"io/fs"
	"os"
	"path"
	"time"

	"github.com/stregato/bao/lib/core"
	"gopkg.in/yaml.v2"
)

type Source struct {
	Name   string
	Data   []byte
	Reader io.Reader
	Size   int64
}

const SizeAll = -1

type ListOption uint32

const (
	// IncludeHiddenFiles includes hidden files in a list operation
	IncludeHiddenFiles ListOption = 1
)

type Range struct {
	From int64
	To   int64
}

type Filter struct {
	Prefix      string                 //Prefix filters on results starting with prefix
	Suffix      string                 //Suffix filters on results ending with suffix
	AfterName   string                 //After ignore all results before the provided one and the provided one
	After       time.Time              //After ignore all results before the provided one and the provided one
	MaxResults  int64                  //MaxResults limits the number of results returned
	OnlyFiles   bool                   //OnlyFiles returns only files
	OnlyFolders bool                   //OnlyFolders returns only folders
	Function    func(fs.FileInfo) bool //Function filters on a custom function
}

type Description struct {
	ReadCost  float64 //ReadCost is the cost of reading 1 byte in CHF as per 2023
	WriteCost float64 //WriteCost is the cost of writing 1 byte in CHF as per 2023
}

// Store is a low level interface to storage services such as S3 or SFTP
type Store interface {
	//ReadDir returns the entries of a folder content
	ReadDir(name string, filter Filter) ([]fs.FileInfo, error)

	// Read reads data from a file into a writer
	Read(name string, rang *Range, dest io.Writer, progress chan int64) error

	// Write writes data to a file name. An existing file is overwritten
	Write(name string, source io.ReadSeeker, progress chan int64) error

	// Stat provides statistics about a file
	Stat(name string) (os.FileInfo, error)

	// Delete deletes a file
	Delete(name string) error

	//ID returns an identifier for the store, typically the URL without credentials information and other parameters
	ID() string

	// Close releases resources
	Close() error

	// String returns a human-readable representation of the storer (e.g. sftp://user@host.cc/path)
	String() string

	//	LastChange(dir string) (time.Time, error)

	Describe() Description
}

func LoadTestURLs() (urls map[string]StoreConfig) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(core.Errorw("cannot get user home dir", err))
	}
	filename := path.Join(homeDir, "credentials.yaml")
	_, err = os.Stat(filename)
	if err != nil {
		filename = "../credentials.yaml"
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		panic(core.Errorw("cannot read file %s", filename, err))
	}

	err = yaml.Unmarshal(data, &urls)
	if err != nil {
		panic(core.Errorw("cannot parse file %s", filename, err))
	}
	return urls
}

func NewTestStore(id string) Store {

	c := LoadTestConfig(nil, id)
	store, err := Open(c)
	if err != nil {
		panic(core.Errorw("cannot open syestore %s", id, err))
	}
	ls, _ := store.ReadDir("", Filter{})
	for _, l := range ls {
		store.Delete(l.Name())
	}

	return store
}
