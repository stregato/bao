//go:build ios && cgo
// +build ios,cgo

package core

/*
#cgo CFLAGS: -x objective-c -fmodules
#cgo LDFLAGS: -framework Foundation
#import <Foundation/Foundation.h>

void iosLog(const char* s) { NSLog(@"%s", s); }
*/
import "C"
import (
	"bytes"
	"sync"
	"unsafe"

	"github.com/sirupsen/logrus"
)

type nslogWriter struct{}

// Write implements io.Writer; trims trailing newline to avoid blank lines in NSLog.
func (nslogWriter) Write(p []byte) (int, error) {
	msg := bytes.TrimRight(p, "\r\n")
	cs := C.CString(string(msg))
	C.iosLog(cs)
	C.free(unsafe.Pointer(cs))
	return len(p), nil
}

// Init routes logrus output to iOS NSLog and disables ANSI colors.
func Init() {
	logrus.SetOutput(nslogWriter{})

	msg := C.CString("Logging initialized with iOS NSLog")
	defer C.free(unsafe.Pointer(msg))
	C.iosLog(msg)
}

var iosLogOnce sync.Once

func init() {
	iosLogOnce.Do(Init)
}
