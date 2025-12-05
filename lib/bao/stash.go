package bao

import (
	"sync"
	"time"

	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/storage"
)

const BlockChainFolder = "blockchain"
const DataFolder = "data"

const (
	ErrNotAuthorized = "not authorized"
)

type Bao struct {
	Id               string             `json:"id"`           // Unique identifier for the stash, derived from URL and public ID
	UserId           security.PrivateID `json:"userId"`       // User's private ID, used for operations that require user authentication
	UserPublicId     security.PublicID  `json:"userPublicId"` // User's public ID, used for public operations and access control
	UserPublicIdHash uint64             `json:"-"`            // Hash of the public ID, used for quick lookups and comparisons
	Url              string             `json:"url"`          // URL of the storage backend, where the stash data is stored
	Author           security.PublicID  `json:"author"`       // Author of the stash, typically the public ID of the user who created it
	DB               *sqlx.DB           `json:"-"`            // Database connection for storing and retrieving stash metadata
	Config           Config             `json:"config"`       // Configuration settings for the stash, including retention policies and storage limits

	store              storage.Store // Storage backend for the stash, used for file operations
	allocatedSize      int64         // Total allocated size for the stash, used for tracking storage usage
	housekeepingTicker *time.Ticker  // Ticker for periodic housekeeping

	lastBlockChainSyncAt time.Time // Timestamp of the last blockchain synchronization
	lastCleanupAt        time.Time // Timestamp of the last retention cleanup
	lastDirsSyncAt       time.Time // Timestamp of the last sync operation
	lastFilesSyncAt      time.Time // Timestamp of the last files sync operation

	ioMu                sync.Mutex                // Mutex for synchronizing I/O operations
	ioScheduleMap       map[FileId]chan struct{}  // Map to track scheduled I/O operations by file I
	ioThrottleCh        chan struct{}             // Channel for throttling I/O operations
	ioWritingWgMaps     map[Group]*sync.WaitGroup // WaitGroup for waiting on I/O operations
	ioLastChangeRunning int32
	groups              []Group // List of groups in the stash, used for organizing files and access control
	blockChainMu        sync.Mutex
}

var openedStashes []*Bao
var openedStashesMu sync.Mutex

const (
	DefaultRetention       = 30 * 24 * time.Hour // 30 days
	DefaultMaxStorage      = 100 * 1024 * 1024   // 100 MB
	DefaultSegmentInterval = 24 * time.Hour      // 1 day
)

type IOOption int

const (
	AsyncOperation     IOOption = 1 << iota // Asynchronous operation, do not wait for completion
	ScheduledOperation                      // Scheduled indicates that the operation should be performed at a later time
)

func (s *Bao) scheduleIo(id FileId) *chan struct{} {
	s.ioMu.Lock()
	defer s.ioMu.Unlock()

	if _, exists := s.ioScheduleMap[id]; exists {
		return nil // If a channel for this file ID already exists, return nil
	}

	ch := make(chan struct{})
	s.ioScheduleMap[id] = ch // Store the channel for this file ID
	return &ch
}

func (s *Bao) completeIo(id FileId) {
	s.ioMu.Lock()
	defer s.ioMu.Unlock()

	if ch, exists := s.ioScheduleMap[id]; exists {
		close(ch)                   // Close the channel to signal completion
		delete(s.ioScheduleMap, id) // Remove the channel from the schedule
	}
}

func (s *Bao) waitIo(id FileId, timeout time.Duration) bool {
	s.ioMu.Lock()
	ch, exists := s.ioScheduleMap[id]
	s.ioMu.Unlock()

	if !exists {
		return false // If no channel exists for this file ID, return false
	}

	select {
	case <-ch: // Wait for the channel to be closed
	case <-time.After(timeout): // Timeout after the specified duration
	}
	return true
}
