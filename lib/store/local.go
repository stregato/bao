package store

import (
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stregato/bao/lib/core"
)

type LocalConfig struct {
	Base string `json:"base" yaml:"base"`
}

type Local struct {
	base  string
	id    string
	touch map[string]time.Time
}

func OpenLocal(id string, c LocalConfig) (Store, error) {
	core.Start("Opening local store.with URL: %s", c.Base)
	if c.Base == "" {
		return nil, core.Error(core.FileError, "base path is required for local store", nil)
	}

	core.End("Opened local store.with path: %s", c.Base)
	return &Local{c.Base, id, map[string]time.Time{}}, nil
}

func (l *Local) ID() string {
	return l.id
}

func (l *Local) Read(name string, rang *Range, dest io.Writer, progress chan int64) error {
	core.Start("name %s", name)
	f, err := os.Open(path.Join(l.base, name))
	if os.IsNotExist(err) {
		return err
	}
	if err != nil {
		return core.Error(core.FileError, "cannot open file on %v:%v", l, err)
	}

	if rang == nil {
		_, err = io.Copy(dest, f)
	} else {
		left := rang.To - rang.From
		f.Seek(rang.From, 0)
		var b [4096]byte

		for left > 0 && err == nil {
			var sz int64
			if rang.From-rang.To > 4096 {
				sz = 4096
			} else {
				sz = rang.From - rang.To
			}
			_, err = f.Read(b[0:sz])
			dest.Write(b[0:sz])
			left -= sz
		}
	}
	if err != nil {
		logrus.Errorf("Cannot read from file: %v", err)
		return core.Error(core.GenericError, "cannot read from %s/%s:%v", l, name, err)
	}

	core.End("Successfully read file: %s", name)
	return nil
}

func createDir(n string) error {
	core.Start("Creating directory: %s", n)
	err := os.MkdirAll(filepath.Dir(n), 0755)
	if err != nil {
		return core.Error(core.FileError, "cannot create directory %s", filepath.Dir(n), err)
	}
	core.End("successfully created directory: %s", filepath.Dir(n))
	return nil
}

func (l *Local) Write(name string, source io.ReadSeeker, progress chan int64) error {
	core.Start("Writing file: %s", name)
	n := filepath.Join(l.base, name)
	err := createDir(n)
	if err != nil {
		return core.Error(core.GenericError, "cannot create parent of %s", n, err)
	}

	f, err := os.Create(n)
	if err != nil {
		return core.Error(core.FileError, "cannot create file on %v:%v", l, err)
	}
	defer f.Close()

	sz, err := io.Copy(f, source)
	if err != nil {
		os.Remove(n)
		return core.Error(core.FileError, "cannot copy file on %v:%v", l, err)
	}

	if progress != nil {
		progress <- sz
	}

	core.End("wrote %d bytes to file %s", sz, n)
	return err
}

func (l *Local) ReadDir(dir string, filter Filter) ([]fs.FileInfo, error) {
	core.Start("Reading directory: %s", dir)
	result, err := os.ReadDir(filepath.Join(l.base, dir))
	if os.IsNotExist(err) {
		return nil, err
	}
	if err != nil {
		return nil, core.Error(core.FileError, "cannot read directory %s", dir, err)
	}

	var infos []fs.FileInfo
	var cnt int64
	for _, item := range result {
		info, err := item.Info()
		if err == nil && matchFilter(info, filter) {
			infos = append(infos, info)
			cnt++
		}
		if filter.MaxResults > 0 && cnt == filter.MaxResults {
			break
		}
	}

	core.End("read %d files from directory %s", len(infos), dir)
	return infos, nil
}

func (l *Local) Stat(name string) (os.FileInfo, error) {
	core.Start("Getting file info for: %s", name)
	f, err := os.Stat(path.Join(l.base, name))
	if os.IsNotExist(err) {
		return nil, err
	}
	if err != nil {
		return nil, core.Error(core.FileError, "cannot stat file on %v:%v", l, name, err)
	}
	core.End("got file info for: %s", name)
	return f, nil
}

func (l *Local) Rename(old, new string) error {
	core.Start("Renaming file from %s to %s", old, new)
	err := os.Rename(path.Join(l.base, old), path.Join(l.base, new))
	if err != nil {
		return core.Error(core.FileError, "cannot rename file from %s to %s on %v:%v", old, new, l.base, l.id, err)
	}
	core.End("renamed file from %s to %s", old, new)
	return nil
}

func (l *Local) Delete(name string) error {
	core.Start("Deleting file: %s", name)
	err := os.RemoveAll(path.Join(l.base, name))
	if err != nil {
		return core.Error(core.DbError, "cannot delete file %s on %v:%v", name, l.base, l.id, err)
	}
	core.End("deleted file %s", name)
	return err
}

func (l *Local) Close() error {
	core.Info("closing local store")
	return nil
}

func (l *Local) Describe() Description {
	return Description{
		ReadCost:  0.0000000001,
		WriteCost: 0.0000000001,
	}
}

func (l *Local) String() string {
	return l.id
}
