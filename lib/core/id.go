package core

import (
	"strconv"
	"time"

	"github.com/godruoyi/go-snowflake"
)

const (
	ReservedBit  uint64 = 1 << 63
	MaxTimestamp uint64 = 1<<41 - 1
	MaxHash      uint64 = 1<<16 - 1
	MaxSequence  uint64 = 1<<6 - 1
)

func SnowID() uint64 {
	return snowflake.ID()
}

func SnowIDString() string {
	return strconv.FormatUint(SnowID(), 16)
}

func TimeFromID(id uint64) time.Time {
	// Retrieve the timestamp part from the identifier by right-shifting and applying a mask
	timestamp := (id >> 22) & MaxTimestamp

	// Convert to a time.Time, note that the timestamp is in milliseconds
	return time.Unix(0, int64(timestamp)*int64(time.Millisecond))
}
