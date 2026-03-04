package vault

import "io"

type readSeekCloser interface {
	io.ReadSeeker
	io.Closer
}
