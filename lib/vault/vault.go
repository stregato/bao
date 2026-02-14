package vault

import (
	"sync"
	"time"

	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/store"
)

const BlockChainFolder = "blockchain"
const DataFolder = "data"

const (
	ErrAccessDenied = "Access denied"
)

type Config struct {
	SyncRelay            string        `json:"syncRelay"`            // Watch service URL for changes notifications
	Retention            time.Duration `json:"retention"`            // How long data is kept
	MaxStorage           int64         `json:"maxStorage"`           // Maximum allowed store.(bytes)
	SegmentInterval      time.Duration `json:"segmentInterval"`      // Time duration of each batch segment
	SyncCooldown         time.Duration `json:"syncCooldown"`         // Minimum time between two sync operations (default 5 seconds)
	WaitTimeout          time.Duration `json:"waitTimeout"`          // Maximum time to wait for I/O operations to complete (default 10 minutes)
	FilesSyncPeriod      time.Duration `json:"filesSyncPeriod"`      // How often to sync files (default 10 minutes)
	CleanupPeriod        time.Duration `json:"cleanupPeriod"`        // How often to run housekeeping (default 1 hour)
	BlockChainSyncPeriod time.Duration `json:"blockChainSyncPeriod"` // How often to sync the blockchain (default 10 minutes)
	IoThrottle           int64         `json:"ioThrottle"`           // Maximum number of concurrent I/O operations. Default is 10.
}

type Vault struct {
	ID         string             `json:"id"`         // Unique identifier for the vault, derived from URL and public ID
	UserSecret security.PrivateID `json:"userSecret"` // User's private ID, used for operations that require user authentication
	UserID     security.PublicID  `json:"userId"`     // User's public ID, used for public operations and access control
	UserIDHash uint64             `json:"-"`          // Hash of the public ID, used for quick lookups and comparisons
	Author     security.PublicID  `json:"author"`     // Author of the vault, typically the public ID of the user who created it
	DB         *sqlx.DB           `json:"-"`          // Database connection for storing and retrieving vault metadata
	Config     Config             `json:"config"`     // Configuration settings for the vault, including retention policies and store.limits
	Realm      Realm              `json:"realm"`      // Realm associated with the vault, used for namespacing and organization

	store              store.Store  // Storage backend for the vault, used for file operations
	allocatedSize      int64        // Total allocated size for the vault, used for tracking store.usage
	housekeepingTicker *time.Ticker // Ticker for periodic housekeeping
	syncRelayCh        chan string  // Channel for receiving sync relay notifications, used to trigger synchronization when changes are detected

	lastBlockChainSyncAt time.Time  // Timestamp of the last blockchain synchronization
	lastCleanupAt        time.Time  // Timestamp of the last retention cleanup
	lastSyncAt           time.Time  // Timestamp of the last sync operation
	lastWaitFilesAt      time.Time  // Timestamp of the last files sync operation
	newFiles             *sync.Cond // Condition variable for signaling changes in watched folders

	ioMu                sync.Mutex               // Mutex for synchronizing I/O operations
	ioScheduleMap       map[FileId]chan struct{} // Map to track scheduled I/O operations by file I
	ioThrottleCh        chan struct{}            // Channel for throttling I/O operations
	ioWritingWg         sync.WaitGroup           // WaitGroup for waiting on I/O operations
	ioLastChangeRunning int32
	blockChainMu        sync.Mutex
}

var openedStashes []*Vault
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

func (v *Vault) scheduleIo(id FileId) *chan struct{} {
	v.ioMu.Lock()
	defer v.ioMu.Unlock()

	if _, exists := v.ioScheduleMap[id]; exists {
		return nil // If a channel for this file ID already exists, return nil
	}

	ch := make(chan struct{})
	v.ioScheduleMap[id] = ch // Store the channel for this file ID
	return &ch
}

func (v *Vault) completeIo(id FileId) {
	v.ioMu.Lock()
	defer v.ioMu.Unlock()

	if ch, exists := v.ioScheduleMap[id]; exists {
		close(ch)                   // Close the channel to signal completion
		delete(v.ioScheduleMap, id) // Remove the channel from the schedule
	}
}

func (v *Vault) waitIo(id FileId, timeout time.Duration) bool {
	v.ioMu.Lock()
	ch, exists := v.ioScheduleMap[id]
	v.ioMu.Unlock()

	if !exists {
		return false // If no channel exists for this file ID, return false
	}

	select {
	case <-ch: // Wait for the channel to be closed
	case <-time.After(timeout): // Timeout after the specified duration
	}
	return true
}
