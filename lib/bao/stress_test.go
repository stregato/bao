package bao

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"gopkg.in/yaml.v2"
)

const stressDefaultStoreURLLabel = "s3"

func newStressTestStash(t *testing.T, cfg Config) (*Bao, *sqlx.DB) {
	t.Helper()

	db := sqlx.NewTestDB(t, "stash.db", "")

	storeURL := loadStoreURL(t, stressDefaultStoreURLLabel)

	owner := security.NewPrivateIDMust()
	s, err := Create(db, owner, storeURL, cfg)
	core.TestErr(t, err, "Create failed: %v")

	t.Cleanup(func() {
		s.Close()
		db.Close()
	})

	return s, db
}

func makePayload(t *testing.T, dir, name string, size int) string {
	t.Helper()
	path := filepath.Join(dir, name)
	data := make([]byte, size)
	for i := range data {
		data[i] = byte('a' + (i % 26))
	}
	core.TestErr(t, os.WriteFile(path, data, 0o644), "WriteFile failed: %v")
	return path
}

// deterministicPayloadSize produces a repeatable size within [base, base+range).
func deterministicPayloadSize(base, sizeRange int, seeds ...int) int {
	if sizeRange <= 0 {
		return base
	}
	acc := 0
	for _, s := range seeds {
		acc = acc*37 + s + 7
	}
	offset := acc % sizeRange
	if offset < 0 {
		offset += sizeRange
	}
	return base + offset
}

func bytesToMB(b int64) float64 {
	return float64(b) / (1024.0 * 1024.0)
}

func mbPerSec(b int64, d time.Duration) float64 {
	if d <= 0 {
		return 0
	}
	return bytesToMB(b) / d.Seconds()
}

func statsFromReadDir(t *testing.T, s *Bao, dir string) ([]File, error) {
	t.Helper()
	files, err := s.ReadDir(dir, time.Time{}, 0, 0)
	return files, err
}

func TestConcurrentWritesStress(t *testing.T) {
	cfg := Config{
		SegmentInterval: 2 * time.Minute,
	}
	stash, _ := newStressTestStash(t, cfg)

	const (
		writers          = 12
		filesPerWriter   = 25
		payloadBaseSize  = 2 * 1024
		payloadSizeRange = 3 * 1024
	)

	sourceDir := t.TempDir()
	var wg sync.WaitGroup
	var idsMu sync.Mutex
	var totalWritten atomic.Int64
	ids := make([]FileId, 0, writers*filesPerWriter)
	errCh := make(chan error, writers*filesPerWriter)

	t.Logf("starting concurrent write stress: writers=%d filesPerWriter=%d payloadRange=[%d,%d) url=%s label=%s", writers, filesPerWriter, payloadBaseSize, payloadBaseSize+payloadSizeRange, stash.Url, stressDefaultStoreURLLabel)
	writeStart := time.Now()

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(writer int) {
			defer wg.Done()
			for i := 0; i < filesPerWriter; i++ {
				name := fmt.Sprintf("stress-%02d-%03d.bin", writer, i)
				payloadSize := deterministicPayloadSize(payloadBaseSize, payloadSizeRange, writer, i)
				totalWritten.Add(int64(payloadSize))
				src := makePayload(t, sourceDir, fmt.Sprintf("src-%02d-%03d", writer, i), payloadSize)
				file, err := stash.Write(name, src, Admins, nil, AsyncOperation, nil)
				if err != nil {
					errCh <- err
					return
				}
				idsMu.Lock()
				ids = append(ids, file.Id)
				idsMu.Unlock()
			}
		}(w)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		core.TestErr(t, err, "Write failed: %v")
	}

	core.TestErr(t, stash.WaitFiles(ids...), "WaitFiles failed: %v")
	writeDur := time.Since(writeStart)

	files, err := statsFromReadDir(t, stash, "")
	core.TestErr(t, err, "ReadDir failed: %v")
	core.Assert(t, len(files) == writers*filesPerWriter, "expected %d files, got %d", writers*filesPerWriter, len(files))

	expectedAllocated, err := stash.calculateAllocatedSize()
	core.TestErr(t, err, "calculateAllocatedSize failed: %v")
	core.Assert(t, stash.AllocatedSize() == expectedAllocated, "AllocatedSize mismatch: have %d want %d", stash.AllocatedSize(), expectedAllocated)

	t.Logf("concurrent write stress done in %s, wrote %d files, %.2f MB (%.2f MB/s)", writeDur, len(files), bytesToMB(totalWritten.Load()), mbPerSec(totalWritten.Load(), writeDur))
}

func TestConcurrentReadWriteStress(t *testing.T) {
	cfg := Config{SegmentInterval: 2 * time.Minute}
	stash, _ := newStressTestStash(t, cfg)

	const (
		baseFiles        = 40
		payloadBaseSize  = 1024
		payloadSizeRange = 2048
		readerCount      = 10
		writerCount      = 10
		iterations       = 30
	)

	sourceDir := t.TempDir()
	var initWritten atomic.Int64

	names := make([]string, baseFiles)
	for i := 0; i < baseFiles; i++ {
		name := fmt.Sprintf("mixed-%03d.dat", i)
		payloadSize := deterministicPayloadSize(payloadBaseSize, payloadSizeRange, i)
		src := makePayload(t, sourceDir, fmt.Sprintf("init-%03d", i), payloadSize)
		initWritten.Add(int64(payloadSize))
		_, err := stash.Write(name, src, Admins, nil, 0, nil)
		core.TestErr(t, err, "initial write failed: %v")
		names[i] = name
	}
	core.TestErr(t, stash.WaitFiles(), "initial WaitFiles failed: %v")
	t.Logf("initialized mixed workload: baseFiles=%d payloadRange=[%d,%d) initWritten=%.2f MB url=%s label=%s", baseFiles, payloadBaseSize, payloadBaseSize+payloadSizeRange, bytesToMB(initWritten.Load()), stash.Url, stressDefaultStoreURLLabel)

	writerWG := sync.WaitGroup{}
	readerWG := sync.WaitGroup{}
	writeErrCh := make(chan error, writerCount*iterations)
	readErrCh := make(chan error, readerCount*iterations)
	writeIDs := make([]FileId, 0, writerCount*iterations)
	var idsMu sync.Mutex
	var totalWritten atomic.Int64
	var totalRead atomic.Int64

	runStart := time.Now()

	for w := 0; w < writerCount; w++ {
		writerWG.Add(1)
		go func(idx int) {
			defer writerWG.Done()
			for i := 0; i < iterations; i++ {
				name := names[(idx+i)%len(names)]
				payloadSize := deterministicPayloadSize(payloadBaseSize, payloadSizeRange, idx, i)
				totalWritten.Add(int64(payloadSize))
				src := makePayload(t, sourceDir, fmt.Sprintf("writer-%02d-%03d", idx, i), payloadSize)
				file, err := stash.Write(name, src, Admins, nil, AsyncOperation, nil)
				if err != nil {
					writeErrCh <- err
					return
				}
				idsMu.Lock()
				writeIDs = append(writeIDs, file.Id)
				idsMu.Unlock()
			}
		}(w)
	}

	readerTmp := t.TempDir()
	var readSuccess atomic.Int64

	for r := 0; r < readerCount; r++ {
		readerWG.Add(1)
		go func(idx int) {
			defer readerWG.Done()
			for i := 0; i < iterations; i++ {
				name := names[(idx*iterations+i)%len(names)]
				dest := filepath.Join(readerTmp, fmt.Sprintf("read-%02d-%03d", idx, i))
				file, err := stash.Read(name, dest, 0, nil)
				if err != nil {
					readErrCh <- err
					return
				}
				if _, err := os.Stat(dest); err != nil {
					readErrCh <- err
					return
				}
				if info, err := os.Stat(dest); err == nil {
					totalRead.Add(info.Size())
				}
				if file.Name == name {
					readSuccess.Add(1)
				}
			}
		}(r)
	}

	writerWG.Wait()
	readerWG.Wait()
	close(writeErrCh)
	close(readErrCh)

	for err := range writeErrCh {
		core.TestErr(t, err, "concurrent write failed: %v")
	}
	for err := range readErrCh {
		core.TestErr(t, err, "concurrent read failed: %v")
	}

	core.Assert(t, readSuccess.Load() == int64(readerCount*iterations), "unexpected read success count: %d", readSuccess.Load())

	if len(writeIDs) > 0 {
		core.TestErr(t, stash.WaitFiles(writeIDs...), "WaitFiles for concurrent writes failed: %v")
	} else {
		core.TestErr(t, stash.WaitFiles(), "WaitFiles for concurrent writes failed: %v")
	}
	runDur := time.Since(runStart)

	files, err := statsFromReadDir(t, stash, "")
	core.TestErr(t, err, "ReadDir after mixed load failed: %v")
	core.Assert(t, len(files) == baseFiles, "expected %d files after mixed load, got %d", baseFiles, len(files))

	totalWriteBytes := initWritten.Load() + totalWritten.Load()
	t.Logf("mixed read/write stress done in %s: writes=%.2f MB (%.2f MB/s) reads=%.2f MB (%.2f MB/s) ops writers=%d readers=%d iterations=%d", runDur, bytesToMB(totalWriteBytes), mbPerSec(totalWriteBytes, runDur), bytesToMB(totalRead.Load()), mbPerSec(totalRead.Load(), runDur), writerCount, readerCount, iterations)
}

func TestRetentionCleanupStress(t *testing.T) {
	cfg := Config{
		Retention:       2 * time.Hour,
		SegmentInterval: 2 * time.Minute,
	}
	stash, db := newStressTestStash(t, cfg)

	if !strings.HasPrefix(stash.Url, "file://") {
		t.Skip("Retention stress test requires file:// store URL")
	}

	const (
		totalFiles = 24
		payload    = 512
	)

	sourceDir := t.TempDir()
	baseStorePath := fileStoreRoot(stash.Url)

	type fileInfo struct {
		id        FileId
		storeDir  string
		storeName string
		name      string
	}

	infos := make([]fileInfo, 0, totalFiles)
	for i := 0; i < totalFiles; i++ {
		name := fmt.Sprintf("retain-%03d.dat", i)
		src := makePayload(t, sourceDir, fmt.Sprintf("retain-src-%03d", i), payload)
		file, err := stash.Write(name, src, Admins, nil, 0, nil)
		core.TestErr(t, err, "initial retention write failed: %v")
		infos = append(infos, fileInfo{id: file.Id, storeDir: file.StoreDir, storeName: file.StoreName, name: name})
	}
	core.TestErr(t, stash.WaitFiles(), "WaitFiles failed for retention setup: %v")

	retentionThreshold := core.Now().Add(-stash.Config.Retention).Add(-1 * time.Hour)
	oldStoreTS := retentionThreshold.Add(-1 * time.Hour).UTC().Format("20060102150405")
	oldStoreDir := filepath.Join(DataFolder, string(Admins), oldStoreTS)
	oldDirPath := filepath.Join(baseStorePath, oldStoreDir)
	core.TestErr(t, os.MkdirAll(filepath.Join(oldDirPath, "h"), 0o755), "mkdir old head failed: %v")
	core.TestErr(t, os.MkdirAll(filepath.Join(oldDirPath, "b"), 0o755), "mkdir old body failed: %v")

	oldCount := totalFiles / 2
	for i := 0; i < oldCount; i++ {
		info := infos[i]
		srcHead := filepath.Join(baseStorePath, info.storeDir, "h", info.storeName)
		srcBody := filepath.Join(baseStorePath, info.storeDir, "b", info.storeName)
		dstHead := filepath.Join(oldDirPath, "h", info.storeName)
		dstBody := filepath.Join(oldDirPath, "b", info.storeName)
		core.TestErr(t, os.Rename(srcHead, dstHead), "move head failed: %v")
		core.TestErr(t, os.Rename(srcBody, dstBody), "move body failed: %v")

		oldMod := retentionThreshold.Add(-time.Hour).UnixMilli()
		_, err := db.Exec("SQL:UPDATE files SET storeDir = :storeDir, modTime = :modTime WHERE store = :store AND id = :id", sqlx.Args{
			"store":    stash.Id,
			"storeDir": oldStoreDir,
			"modTime":  oldMod,
			"id":       info.id,
		})
		core.TestErr(t, err, "update old file metadata failed: %v")
	}

	stash.retentionCleanup()

	files, err := statsFromReadDir(t, stash, "")
	core.TestErr(t, err, "ReadDir after retention cleanup failed: %v")
	core.Assert(t, len(files) == totalFiles-oldCount, "expected %d files after cleanup, got %d", totalFiles-oldCount, len(files))

	if _, err := os.Stat(oldDirPath); !os.IsNotExist(err) {
		t.Fatalf("expected directory %s to be removed", oldDirPath)
	}

	var countRow int
	row := db.Engine.QueryRow("SELECT COUNT(*) FROM files WHERE store = ?", stash.Id)
	core.TestErr(t, row.Scan(&countRow), "count remaining files failed: %v")
	core.Assert(t, countRow == totalFiles-oldCount, "expected %d rows in files table, got %d", totalFiles-oldCount, countRow)

	dbAllocated, err := stash.calculateAllocatedSize()
	core.TestErr(t, err, "calculateAllocatedSize failed")
	core.Assert(t, stash.AllocatedSize() == dbAllocated, "allocated size mismatch: %d vs %d", stash.AllocatedSize(), dbAllocated)
}

func fileStoreRoot(url string) string {
	root := strings.TrimPrefix(url, "file://")
	if runtime.GOOS == "windows" && strings.HasPrefix(root, "/") {
		root = root[1:]
	}
	return root
}

func loadStoreURL(t *testing.T, label string) string {
	t.Helper()

	_, caller, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("unable to determine caller information for store URL lookup")
	}

	urlFile := filepath.Join(filepath.Dir(caller), "..", "..", "credentials", "urls.yaml")
	data, err := os.ReadFile(urlFile)
	if err != nil {
		t.Fatalf("failed to read store URLs from %s: %v", urlFile, err)
	}

	urls := map[string]string{}
	if err := yaml.Unmarshal(data, &urls); err != nil {
		t.Fatalf("failed to parse store URLs in %s: %v", urlFile, err)
	}

	url, ok := urls[label]
	if !ok || strings.TrimSpace(url) == "" {
		t.Fatalf("store URL for label %q not found in %s", label, urlFile)
	}

	return url
}
