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

// Error code constants for semantic error categorization
const (
	DbError      = "DbError"
	FileError    = "FileError"
	ParseError   = "ParseError"
	EncodeError  = "EncodeError"
	AuthError    = "AuthError"
	AccessDenied = "AccessDenied"
	NetError     = "NetError"
	ConfigError  = "ConfigError"
	TestError    = "TestError"
	GenericError = "GenericError"
	Timeout      = "Timeout"
)

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
	err := Error(GenericError, format, args...)
	logrus.Error(stacktraceString(err))
}

type wrappedError struct {
	Code  string `json:"code"`  // The error code
	Msg   string `json:"msg"`   // The error message
	File  string `json:"file"`  // The file where the error occurred
	Line  int    `json:"line"`  // The line number where the error occurred
	Cause error  `json:"cause"` // The underlying cause of the error
}

func (e *wrappedError) Error() string { return e.Msg }
func (e *wrappedError) Unwrap() error { return e.Cause }
func ErrorCode(err error) string {
	if we, ok := err.(*wrappedError); ok {
		return we.Code
	}
	return ""
}

func Error(code string, format string, args ...any) error {
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
	if cause != nil && code == GenericError {
		if we, ok := cause.(*wrappedError); ok {
			code = we.Code
		}
	}
	msg := fmt.Sprintf(format, args...)
	log := fmt.Sprintf("ERROR [%s] %s:%d - %s", fn, filepath.Base(file), line, msg)
	logrus.Errorf(log)
	addRecentLog(log)
	return &wrappedError{
		Code:  code,
		File:  base,
		Line:  line,
		Msg:   msg,
		Cause: cause,
	}
}

func (e *wrappedError) MarshalJSON() ([]byte, error) {
	type jsonErr struct {
		Code    string   `json:"code"`
		Message string   `json:"msg,omitempty"`
		File    string   `json:"file,omitempty"`
		Line    int      `json:"line,omitempty"`
		Cause   *jsonErr `json:"cause,omitempty"`
	}

	var encode func(error) *jsonErr
	encode = func(err error) *jsonErr {
		if err == nil {
			return nil
		}
		if we, ok := err.(*wrappedError); ok {
			return &jsonErr{
				Code:    we.Code,
				Message: we.Msg,
				File:    we.File,
				Line:    we.Line,
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
			fmt.Fprintf(&b, "%s - %s:%d: %s\n\t", we.Code, we.File, we.Line, we.Msg)
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
		args = append(args, err)
		msg = fmt.Sprintf(msg, args...)
		err = Error(TestError, msg, err)
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
