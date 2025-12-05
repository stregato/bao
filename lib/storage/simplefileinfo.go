package storage

import (
	"io/fs"
	"strings"
	"time"
)

func matchFilter(f fs.FileInfo, filter Filter) bool {
	name := f.Name()
	return strings.HasPrefix(name, filter.Prefix) &&
		strings.HasSuffix(name, filter.Suffix) &&
		(filter.After.IsZero() || f.ModTime().After(filter.After)) &&
		name > filter.AfterName &&
		(!filter.OnlyFiles || !f.IsDir()) &&
		(!filter.OnlyFolders || f.IsDir())
}

type simpleFileInfo struct {
	name    string
	size    int64
	isDir   bool
	modTime time.Time
	mode    uint32
}

func NewSimpleFileInfo(name string, size int64, isDir bool, modTime time.Time, mode uint32) fs.FileInfo {
	return simpleFileInfo{
		name:    name,
		size:    size,
		isDir:   isDir,
		modTime: modTime,
		mode:    mode,
	}
}

func (f simpleFileInfo) Name() string {
	return f.name
}

func (f simpleFileInfo) Size() int64 {
	return f.size
}

func (f simpleFileInfo) Mode() fs.FileMode {
	return 0644
}

func (f simpleFileInfo) ModTime() time.Time {
	return f.modTime
}

func (f simpleFileInfo) IsDir() bool {
	return f.isDir
}

func (f simpleFileInfo) Sys() interface{} {
	return nil
}
