package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/stregato/bao/lib/core"
	"github.com/vmihailenco/msgpack/v5"
)

func ReadFile(s Store, name string) ([]byte, error) {
	var b bytes.Buffer
	err := s.Read(name, nil, &b, nil)
	return b.Bytes(), err
}

func WriteFile(s Store, name string, data []byte) error {
	b := core.NewBytesReader(data)
	defer b.Close()
	return s.Write(name, b, nil)
}

func ReadJSON(s Store, name string, v any, hash hash.Hash) error {
	data, err := ReadFile(s, name)
	if err == nil {
		if hash != nil {
			hash.Write(data)
		}

		err = json.Unmarshal(data, v)
	}
	return err
}

func WriteJSON(s Store, name string, v any, hash hash.Hash) error {
	b, err := json.Marshal(v)
	if err == nil {
		if hash != nil {
			hash.Write(b)
		}
		err = s.Write(name, core.NewBytesReader(b), nil)
	}
	return err
}

func ReadMsgPack(s Store, name string, v any) error {
	data, err := ReadFile(s, name)
	if os.IsNotExist(err) {
		return err
	}
	if err != nil {
		return core.Errorw("msgpackErr: cannot read file %s from store %s", name, s, err)
	}
	err = msgpack.Unmarshal(data, v)
	if err != nil {
		return core.Errorw("msgpackErr: cannot unmarshal msgpack file %s from store %s ", name, s, err)
	}

	return err
}

func WriteMsgPack(s Store, name string, v any) error {
	b, err := msgpack.Marshal(v)
	if err != nil {
		return core.Errorw("msgpackErr: cannot marshal in store %s msgpack file %s", s, name, err)
	}
	err = s.Write(name, core.NewBytesReader(b), nil)
	if err != nil {
		return core.Errorw("msgpackErr: cannot write file %s into store %s", name, s, err)
	}
	return nil
}

func ReadYAML(s Store, name string, v interface{}, hash hash.Hash) error {
	data, err := ReadFile(s, name)
	if err == nil {
		if hash != nil {
			hash.Write(data)
		}

		err = yaml.Unmarshal(data, v)
	}
	return err
}

func WriteYAML(s Store, name string, v interface{}, hash hash.Hash) error {
	b, err := yaml.Marshal(v)
	if err == nil {
		if hash != nil {
			hash.Write(b)
		}
		err = s.Write(name, core.NewBytesReader(b), nil)
	}
	return err
}

const maxSizeForMemoryCopy = 1024 * 1024

func CopyFile(dest Store, destName string, source Store, sourceName string) error {
	stat, err := source.Stat(sourceName)
	if err != nil {
		return core.Errorw("cannot stat %s/%s", source, sourceName, err)
	}

	var r io.ReadSeeker
	if stat.Size() <= maxSizeForMemoryCopy {
		buf := bytes.Buffer{}
		err = source.Read(sourceName, nil, &buf, nil)
		if err != nil {
			return core.Errorw("cannot read %s/%s", source, sourceName, err)
		}
		r = core.NewBytesReader(buf.Bytes())
	} else {
		file, err := os.CreateTemp("", "woland")
		if err != nil {
			return core.Errorw("cannot create temporary file for CopyFile", err)
		}

		err = source.Read(sourceName, nil, file, nil)
		if err != nil {
			return core.Errorw("cannot read %s/%s", source, sourceName, err)
		}
		file.Seek(0, 0)
		r = file
		defer func() {
			file.Close()
			os.Remove(file.Name())
		}()
	}

	err = dest.Write(destName, r, nil)
	if err != nil {
		dest.Delete(destName)
		return core.Errorw("cannot write %s/%s", dest, destName, err)
	}

	return nil
}

func Dump(store Store, dir string, content bool) string {
	var builder strings.Builder
	files, err := store.ReadDir(dir, Filter{})
	if err != nil {
		return ""
	}
	var subdirs []string
	for _, file := range files {
		if file.IsDir() {
			subdir := filepath.Join(dir, file.Name())
			subdirs = append(subdirs, subdir)
		}
	}
	sort.Strings(subdirs)
	for _, subdir := range subdirs {
		subdirOutput := Dump(store, subdir, content)
		builder.WriteString(subdirOutput)
	}
	for _, file := range files {
		if !file.IsDir() {
			fmt.Fprintf(&builder, "%s\n", filepath.Join(dir, file.Name()))
			if content {
				data, _ := ReadFile(store, filepath.Join(dir, file.Name()))
				fmt.Fprintf(&builder, "%s\n", string(data))
			}
		}
	}
	return builder.String()
}

func DeleteDir(store Store, dir string) error {
	files, err := store.ReadDir(dir, Filter{})
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.IsDir() {
			err = DeleteDir(store, filepath.Join(dir, file.Name()))
			if err != nil {
				return err
			}
		} else {
			err = store.Delete(filepath.Join(dir, file.Name()))
			if err != nil {
				return err
			}
		}
	}
	return store.Delete(dir)
}

func GetSize(store Store, dir string) (int64, error) {
	var totalSize int64
	files, err := store.ReadDir(dir, Filter{})
	if err != nil {
		return 0, err
	}
	for _, file := range files {
		if file.IsDir() {
			size, err := GetSize(store, filepath.Join(dir, file.Name()))
			if err != nil {
				return 0, err
			}
			totalSize += size
		} else {
			totalSize += file.Size()
		}
	}
	return totalSize, nil
}
