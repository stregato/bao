//go:build !js

package vault

import (
	"io"
	"os"
)

func openLocalCopyWriter(dest string) (io.WriteCloser, error) {
	return os.Create(dest)
}

func cleanupLocalCopy(dest string) {
	_ = os.Remove(dest)
}

func ReadLocalCopyBytes(dest string) ([]byte, error) {
	return os.ReadFile(dest)
}

func RemoveLocalCopy(dest string) {
	cleanupLocalCopy(dest)
}
