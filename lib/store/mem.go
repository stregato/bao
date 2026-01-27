package store

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"time"

	"github.com/stregato/bao/lib/core"
)

type _memoryFile struct {
	simpleFileInfo simpleFileInfo
	content        []byte
}

type Memory struct {
	id   string
	data map[string]_memoryFile
}

var MemoryStores = map[string]*Memory{}

func OpenMemory(id string) (Store, error) {
	if m, ok := MemoryStores[id]; ok {
		return m, nil
	}
	m := &Memory{id: id, data: map[string]_memoryFile{}}
	MemoryStores[id] = m
	return m, nil
}

func (m *Memory) ID() string {
	return m.id
}

func (m *Memory) Read(name string, rang *Range, dest io.Writer, progress chan int64) error {
	f, ok := m.data[name]
	if !ok {
		return os.ErrNotExist
	}

	var err error
	var w int64
	if rang == nil {
		w, err = io.Copy(dest, core.NewBytesReader(f.content))
	} else {
		w, err = io.CopyN(dest, core.NewBytesReader(f.content[rang.From:]), rang.To-rang.From)
	}
	if err != nil {
		return core.Errorw("cannot read from %s/%s:%v", m, name, err)
	}
	if progress != nil {
		progress <- w
	}

	return nil
}

func (m *Memory) Write(name string, source io.ReadSeeker, progress chan int64) error {
	var buf bytes.Buffer

	_, err := io.Copy(&buf, source)
	if err != nil {
		return core.Errorw("cannot copy file '%s'' in memory:%v", name, err)
	}
	content := buf.Bytes()
	if progress != nil {
		progress <- int64(len(content))
	}

	m.data[name] = _memoryFile{
		simpleFileInfo: simpleFileInfo{
			name:    path.Base(name),
			size:    int64(len(content)),
			modTime: core.Now(),
			isDir:   false,
		},
		content: content,
	}

	return err
}

func (m *Memory) ReadDir(dir string, f Filter) ([]fs.FileInfo, error) {
	var infos []fs.FileInfo
	subfolders := map[string]bool{}
	for n, mf := range m.data {
		if strings.HasPrefix(n, dir+"/") {
			n = strings.TrimPrefix(n, dir+"/")
			parts := strings.Split(n, "/")
			if len(parts) > 1 && !f.OnlyFiles {
				subfolders[parts[0]] = true
			} else if matchFilter(mf.simpleFileInfo, f) {
				infos = append(infos, mf.simpleFileInfo)
			}
		}
	}

	for subfolder := range subfolders {
		info := simpleFileInfo{
			name:    subfolder,
			size:    0,
			modTime: core.Now(),
			isDir:   true,
		}
		if matchFilter(info, f) {
			infos = append(infos, info)
		}
	}

	return infos, nil
}

func (m *Memory) Stat(name string) (os.FileInfo, error) {
	l, ok := m.data[name]
	if ok {
		return l.simpleFileInfo, nil
	} else {
		for n := range m.data {
			if strings.HasPrefix(n, name+"/") {
				return simpleFileInfo{
					name:  path.Base(name),
					isDir: true,
				}, nil
			}
		}
		return nil, os.ErrNotExist
	}
}

func (m *Memory) Delete(name string) error {
	_, ok := m.data[name]
	if ok {
		delete(m.data, name)
		return nil
	} else {
		return os.ErrNotExist
	}
}

func (m *Memory) Close() error {
	return nil
}

func (m *Memory) LastChange(dir string) (time.Time, error) {
	return time.Time{}, nil
}

func (m *Memory) Describe() Description {
	return Description{
		ReadCost:  0.0000000001,
		WriteCost: 0.0000000001,
	}
}

func (m *Memory) String() string {
	return m.id
}
