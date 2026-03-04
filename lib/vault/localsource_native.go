//go:build !js

package vault

import (
	"fmt"
	"os"
	"strings"
)

func statLocalSource(source string) (os.FileInfo, error) {
	if strings.HasPrefix(source, "jsblob:") {
		return nil, fmt.Errorf("jsblob source is not supported on native runtime")
	}
	return os.Stat(source)
}

func openLocalSourceReader(source string) (readSeekCloser, error) {
	if strings.HasPrefix(source, "jsblob:") {
		return nil, fmt.Errorf("jsblob source is not supported on native runtime")
	}
	return os.Open(source)
}
