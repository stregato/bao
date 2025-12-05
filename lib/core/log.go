package core

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

var formatter *LogFormatter = &LogFormatter{last: time.Now()}

func init() {
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetFormatter(formatter)
}

type LogFormatter struct {
	last time.Time
	mu   sync.Mutex
}

func TimeTrack() {
	formatter.last = time.Now()
}

func (f *LogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	elapsed := entry.Time.Sub(f.last).Milliseconds()

	timestamp := entry.Time.Format("15:04:05")
	log := fmt.Sprintf("%-5s [%s] (+%dms) %s\n",
		strings.ToUpper(entry.Level.String()),
		timestamp,
		elapsed,
		entry.Message,
	)

	return []byte(log), nil
}

// GetRecentLog returns the last n log lines (or fewer if not enough).
func GetRecentLog(n int) []string {
	if n <= 0 {
		return []string{}
	}
	res := make([]string, 0, n)
	idx := recentLogNext
	for count := 0; count < MaxRecentErrors && len(res) < n; count++ {
		msg := recentLog[idx]
		if msg != "" {
			res = append(res, msg)
		}
		idx = (idx + 1) % MaxRecentErrors
	}

	return res
}

func addRecentLog(msg string) {
	// add the current time to the log message
	msg = fmt.Sprintf("%s - %s", Now().Format("2006-01-02 15:04:05"), msg)

	recentLog[recentLogNext] = msg
	recentLogNext = (recentLogNext + 1) % MaxRecentErrors
	if httpLog != "" {
		httpLogCh <- msg
	}
}

var httpLog string
var httpLogCh chan string = make(chan string, 1000)

func SetHttpLog(url string) {
	httpLog = url

	go func() {
		for data := range httpLogCh {
			resp, err := http.Post(httpLog, "text/plain", strings.NewReader(data))
			if err != nil {
				logrus.WithError(err).Error("http log: post failed")
				continue
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			if resp.StatusCode/100 != 2 {
				logrus.WithField("status", resp.Status).Error("http log: non-2xx")
			}
		}
	}()
	Info("HTTP log set to %s", url)
}

func Trace(format string, args ...any) {
	if logrus.GetLevel() >= logrus.TraceLevel {
		msg := fmt.Sprintf(format, args...)
		pc, file, no, ok := runtime.Caller(1)
		details := runtime.FuncForPC(pc)
		if ok && details != nil {
			msg = fmt.Sprintf("%s[%s:%d] - %s", path.Base(details.Name()), filepath.Base(file), no, msg)
		}
		logrus.Info(msg)
		addRecentLog(msg)
	}
}

func Start(format string, args ...any) {
	if logrus.GetLevel() < logrus.DebugLevel {
		return
	}

	msg := fmt.Sprintf(format, args...)
	pc, file, line, ok := runtime.Caller(1)
	if !ok {
		file, line = "?", 0
	}
	fn := path.Base(runtime.FuncForPC(pc).Name())

	log := fmt.Sprintf("%*sSTART[%s] %s:%d - %s", stackDepth(1), "  ", fn, filepath.Base(file), line, msg)
	logrus.Debug(log)
	addRecentLog(log)

}

func stackDepth(skip int) int {
	if skip < 0 {
		skip = 0
	}
	pcs := make([]uintptr, 32)
	for {
		n := runtime.Callers(skip+1, pcs)
		if n < len(pcs) {
			return n
		}
		pcs = make([]uintptr, len(pcs)*2)
	}
}

func End(format string, args ...any) {
	if logrus.GetLevel() < logrus.DebugLevel {
		return
	}

	msg := fmt.Sprintf(format, args...)
	pc, file, line, ok := runtime.Caller(1)
	if !ok {
		file, line = "?", 0
	}
	fn := path.Base(runtime.FuncForPC(pc).Name())

	log := fmt.Sprintf("%*sEND[%s] %s:%d - %s", stackDepth(1), "  ", fn, filepath.Base(file), line, msg)
	logrus.Debug(log)
	addRecentLog(log)
}

func Info(format string, args ...any) {
	if logrus.GetLevel() >= logrus.InfoLevel {
		msg := fmt.Sprintf(format, args...)
		pc, file, no, ok := runtime.Caller(1)
		details := runtime.FuncForPC(pc)
		if ok && details != nil {
			msg = fmt.Sprintf("%s[%s:%d] - %s", path.Base(details.Name()), filepath.Base(file), no, msg)
		}
		logrus.Info(msg)
		addRecentLog(msg)
	}
}

func Debug(format string, args ...any) {
	if logrus.GetLevel() >= logrus.DebugLevel {
		msg := fmt.Sprintf(format, args...)
		pc, file, no, ok := runtime.Caller(1)
		details := runtime.FuncForPC(pc)
		if ok && details != nil {
			msg = fmt.Sprintf("%s[%s:%d] - %s", path.Base(details.Name()), filepath.Base(file), no, msg)
		}
		logrus.Debug(msg)
		addRecentLog(msg)
	}
}

// IsErr logs an error at error level with stack context and returns true if err is not nil.
// This is intended for inline logging. Prefer returning core.Errorw(...) to propagate errors.
func IsErr(err error, format string, args ...any) bool {
	if err == nil {
		return false
	}
	// Append the error as the last arg for convenience if callers didn't pass it
	msg := getAssertMsg(format, 2, append(args, err)...)
	logrus.Error(msg)
	addRecentLog(msg)
	return true
}

// IsWarn logs an error at warn level and returns true if err is not nil.
// This is intended for inline logging where the error is not propagated.
func IsWarn(err error, format string, args ...any) bool {
	if err == nil {
		return false
	}
	msg := fmt.Sprintf(format, append(args, err)...)
	logrus.Warn(msg)
	addRecentLog(msg)
	return true
}
