package store

import (
	"bytes"
	"path"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stregato/bao/lib/core"
	"github.com/stretchr/testify/assert"
)

func TestS3(t *testing.T) {
	c := LoadTestConfig(t, "s3")
	testStore(t, c)
}

// func TestWebdav(t *testing.T) {
// 	credentials := LoadTestURLs()
// 	testStore(t, credentials["dav"])
// }

func TestLocal(t *testing.T) {
	c := LoadTestConfig(t, "test")
	testStore(t, c)
}

func testStore(t *testing.T, c StoreConfig) {
	s, err := Open(c)
	core.TestErr(t, err, "cannot open store: %v", err)
	defer s.Close()

	testCreateFile(t, s)
	testReadDir(t, s)
	// testReadWrite(t, s)
}

func testCreateFile(t *testing.T, s Store) {
	name := uuid.New().String()
	r := bytes.NewReader(make([]byte, 1024))
	assert.NoErrorf(t, s.Write(name, r, nil), "cannot write file %s", name)
	assert.NoErrorf(t, s.Delete(name), "cannot delete file %s", name)
}

func testReadDir(t *testing.T, s Store) {
	err := s.Delete("ut")
	core.TestErr(t, err, "cannot delete folder: %v", err)

	err = WriteFile(s, "ut/test1.txt", []byte("test1"))
	core.TestErr(t, err, "cannot write file: %v", err)

	time.Sleep(1 * time.Second)
	err = WriteFile(s, "ut/test2.h", []byte("test2"))
	core.TestErr(t, err, "cannot write file: %v", err)

	err = WriteFile(s, "ut/sub/test3.txt", []byte("test3"))
	core.TestErr(t, err, "cannot write file: %v", err)

	files, err := s.ReadDir("ut", Filter{})
	core.TestErr(t, err, "cannot read dir: %v", err)
	core.Assert(t, len(files) == 3, "wrong number of files: %d", len(files))

	files, err = s.ReadDir("ut", Filter{Suffix: ".txt"})
	core.TestErr(t, err, "cannot read dir: %v", err)
	core.Assert(t, len(files) == 1, "wrong number of files: %d", len(files))

	files, err = s.ReadDir("ut", Filter{OnlyFolders: true})
	core.TestErr(t, err, "cannot read dir: %v", err)
	core.Assert(t, len(files) == 1, "wrong number of files: %d", len(files))
	core.Assert(t, files[0].Name() == "sub", "wrong folder name: %s", files[0].Name())

	stat, err := s.Stat("ut/test2.h")
	core.TestErr(t, err, "cannot stat file: %v", err)

	files, err = s.ReadDir("ut", Filter{After: stat.ModTime()})
	core.TestErr(t, err, "cannot read dir: %v", err)
	core.Assert(t, len(files) == 1, "wrong number of files: %d", len(files))

	err = WriteFile(s, "ut/test3.h", []byte("test2"))
	core.TestErr(t, err, "cannot write file: %v", err)

	err = s.Delete("ut/sub")
	core.TestErr(t, err, "cannot delete folder: %v", err)

	files, err = s.ReadDir("ut", Filter{OnlyFolders: true})
	core.TestErr(t, err, "cannot read dir: %v", err)
	core.Assert(t, len(files) == 0, "wrong number of files: %d", len(files))

	err = s.Delete("ut")
	core.TestErr(t, err, "cannot delete folder: %v", err)
	files, err = s.ReadDir("ut", Filter{})
	core.Assert(t, len(files) == 0, "wrong number of files: %d", len(files))

}

func testReadWrite(t *testing.T, s Store) {
	progress := make(chan int64)
	var progressCount int64
	var data [][]byte = make([][]byte, 16)

	go func() {
		for p := range progress {
			progressCount += p
		}
	}()

	for i := 0; i < 16; i++ {
		data[i] = core.GenerateRandomBytes(i * 1024)
		r := core.NewBytesReader(data[i])
		name := path.Join("ut", "item")
		for j := 0; j < i; j++ {
			name = path.Join(name+strconv.Itoa(j), "item")
		}
		err := s.Write(name, r, progress)
		core.TestErr(t, err, "cannot write file: %v", err)
		core.Assert(t, progressCount >= int64(len(data[i])), "wrong progress: %d", progressCount)
		progressCount = 0
	}

	for i := 0; i < 16; i++ {
		name := path.Join("ut", "item")
		for j := 0; j < 16; j++ {
			name = path.Join(name+strconv.Itoa(j), "item")
		}
		var b bytes.Buffer
		err := s.Read(name, nil, &b, progress)
		core.TestErr(t, err, "cannot read file: %v", err)
		core.Assert(t, bytes.Equal(data[i], b.Bytes()), "wrong data")
		core.Assert(t, progressCount >= int64(len(data[i])), "wrong progress: %d", progressCount)
		progressCount = 0

		for j := 0; j < 16; j++ {
			r := Range{
				From: int64(i / 16 * j),
				To:   int64(i / 16 * (j + 1)),
			}
			err := s.Read(name, &r, &b, nil)
			core.TestErr(t, err, "cannot read file: %v", err)
			core.Assert(t, bytes.Equal(data[i][r.From:r.To], b.Bytes()), "wrong data")
		}

		err = s.Delete(name)
		core.TestErr(t, err, "cannot delete file: %v", err)
	}
}
