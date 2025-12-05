package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

var ErrNotInitialized = fmt.Errorf("owndata not initialized")
var ErrNoDriver = fmt.Errorf("no driver found for the provided configuration")
var ErrInvalidSignature = fmt.Errorf("signature does not match the user id")
var ErrInvalidSize = fmt.Errorf("provided slice has not enough data")
var ErrInvalidVersion = fmt.Errorf("version of protocol is not compatible")
var ErrInvalidChangeFilePath = fmt.Errorf("a change file is not in a valid Woland folder")
var ErrInvalidFilePath = fmt.Errorf("a file is not in a valid owndata folder")
var ErrNoExchange = fmt.Errorf("no exchange reachable for the domain")
var ErrNotAuthorized = fmt.Errorf("user is not authorized in the domain")
var ErrInvalidId = fmt.Errorf("the id is invalid")

const MaxRecentErrors = 16000

var MaxStacktraceOut = 30

var recentLog [MaxRecentErrors]string
var recentLogNext int

var T *testing.T

func ErrLike(err error, format string) bool {
	if err == nil {
		return false
	}
	e := err.Error()
	if e == format {
		return true
	}

	f := []rune(format)
	s := []rune(e)

	formatIdx := 0
	errIdx := 0

	for formatIdx < len(f) && errIdx < len(s) {
		if f[formatIdx] == '%' {
			for formatIdx < len(f) && !strings.Contains("sdw", string(f[formatIdx])) {
				formatIdx++
			}
		}
		if s[errIdx] == f[formatIdx] {
			formatIdx++
		}
		errIdx++
	}

	return formatIdx == len(f)
}

func LogError(format string, args ...any) {
	err := Errorw(format, args...)
	logrus.Error(stacktraceString(err))
}

type wrappedError struct {
	File  string `json:"file"`  // The file where the error occurred
	Line  int    `json:"line"`  // The line number where the error occurred
	Msg   string `json:"msg"`   // The error message
	Cause error  `json:"cause"` // The underlying cause of the error
}

func (e *wrappedError) Error() string { return e.Msg }
func (e *wrappedError) Unwrap() error { return e.Cause }

func Errorw(format string, args ...any) error {
	pc, file, line, _ := runtime.Caller(1)
	fn := path.Base(runtime.FuncForPC(pc).Name())
	base := filepath.Base(file)

	var cause error
	if n := len(args); n > 0 {
		if c, ok := args[n-1].(error); ok {
			cause = c
			args = args[:n-1]
		}
	}
	msg := fmt.Sprintf(format, args...)
	log := fmt.Sprintf("ERROR [%s] %s:%d - %s", fn, filepath.Base(file), line, msg)
	logrus.Errorf(log)
	addRecentLog(log)
	return &wrappedError{
		File:  base,
		Line:  line,
		Msg:   msg,
		Cause: cause,
	}
}

func (e *wrappedError) MarshalJSON() ([]byte, error) {
	type jsonErr struct {
		File    string   `json:"file,omitempty"`
		Line    int      `json:"line,omitempty"`
		Message string   `json:"msg"`
		Cause   *jsonErr `json:"cause,omitempty"`
	}

	var encode func(error) *jsonErr
	encode = func(err error) *jsonErr {
		if err == nil {
			return nil
		}
		if we, ok := err.(*wrappedError); ok {
			return &jsonErr{
				File:    we.File,
				Line:    we.Line,
				Message: we.Msg,
				Cause:   encode(we.Cause),
			}
		}
		// Fallback for plain errors
		return &jsonErr{Message: err.Error()}
	}

	return json.Marshal(encode(e))
}

type multiUnwrap interface{ Unwrap() []error }

func stacktraceString(err error) string {
	var b strings.Builder
	var walk func(error)
	walk = func(e error) {
		switch we := e.(type) {
		case *wrappedError:
			fmt.Fprintf(&b, "%s:%d: %s\n\t", we.File, we.Line, we.Msg)
			if we.Cause != nil {
				walk(we.Cause)
			}
		case multiUnwrap:
			children := we.Unwrap()
			for i, c := range children {
				fmt.Fprintf(&b, "[%d] ", i)
				walk(c)
			}
		default:
			fmt.Fprintf(&b, "%v\n", e)
			if u := errors.Unwrap(e); u != nil {
				walk(u)
			}
		}
	}
	walk(err)
	return strings.TrimRight(b.String(), "\n")
}

func TestErr(t *testing.T, err error, msg string, args ...interface{}) {
	if err != nil {
		msg = fmt.Sprintf(msg, args...)
		err = Errorw(msg, err)
		logrus.Fatal(stacktraceString(err))
		t.FailNow()
	}
}

func Assert(t *testing.T, cond bool, msg string, args ...interface{}) {
	if !cond {
		msg = getAssertMsg(msg, 2, args...)
		t.Fatalf("%s", msg)
	}
}

func getAssertMsg(msg string, skip int, args ...interface{}) string {
	msg = fmt.Sprintf(msg, args...)
	for i := skip; i < MaxStacktraceOut; i++ {
		pc, file, no, ok := runtime.Caller(i)
		if !ok {
			break
		}
		details := runtime.FuncForPC(pc)
		name := path.Base(details.Name())
		if strings.HasSuffix(name, "testing.tRunner") {
			break
		}
		if details != nil {
			msg = fmt.Sprintf("%s\n\t%s: %s:%d", msg, name, file, no)
		}
	}
	return msg
}
