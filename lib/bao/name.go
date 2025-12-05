package bao

import (
	"strconv"
	"sync"
	"time"

	"github.com/stregato/bao/lib/core"
)

var (
	lastMs   int64
	seq      uint8
	seqMutex sync.Mutex
	nodeHash = getNodeHash()
)

func getMillisOfDay(t time.Time) int64 {
	return int64(t.Hour()*3600000 + t.Minute()*60000 + t.Second()*1000 + t.Nanosecond()/1e6)
}

func generateFilename(t time.Time) string {
	ms := getMillisOfDay(t)

	seqMutex.Lock()
	if ms != lastMs {
		seq = 0
		lastMs = ms
	} else {
		seq++
	}
	s := seq
	seqMutex.Unlock()

	// Compose a 40-bit int: [27 bits ms] [8 bits node] [5 bits seq]
	id := (ms << 13) | (int64(nodeHash) << 5) | int64(s)

	// Base36 encoding
	return strconv.FormatUint(uint64(id), 36)
}

func getSegmentDir(segmentInterval time.Duration) string {
	core.Start("segmentInterval %s", segmentInterval)
	if segmentInterval <= time.Minute {
		segmentInterval = DefaultSegmentInterval
	}

	segmentTime := core.Now().UTC().Truncate(segmentInterval)
	segmentDir := segmentTime.Format("20060102150405") // YYYY/MM/DD/HH/mm/ss
	core.End("segmentDir %s", segmentDir)
	return segmentDir
}
