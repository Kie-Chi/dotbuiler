package logger

import (
	"fmt"
	"os"
	"strings"
	"time"
	"sync"
)

var (
	debugEnabled bool
	logMu        sync.Mutex
)
// ANSI Color Codes
const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Magenta= "\033[35m"
	Cyan   = "\033[36m"
	Gray   = "\033[90m"
)

func init() {
	v := strings.ToLower(os.Getenv("DOTBUILDER_DEBUG"))
	debugEnabled = v == "1" || v == "true" || v == "yes"
}

func ts() string {
	return Gray + time.Now().Format("15:04:05") + Reset
}

func SetDebug(enable bool) { debugEnabled = enable }

func Info(format string, args ...interface{}) {
	logMu.Lock()
	defer logMu.Unlock()
	prefix := "[INFO]"
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s %s\n", ts(), prefix, msg)
}

func Warn(format string, args ...interface{}) {
	logMu.Lock()
	defer logMu.Unlock()
	prefix := Yellow + "[WARN]" + Reset
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s %s\n", ts(), prefix, msg)
}

func Error(format string, args ...interface{}) {
	logMu.Lock()
	defer logMu.Unlock()
	prefix := Red + "[ERRO]" + Reset
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s %s\n", ts(), prefix, msg)
	os.Exit(1)
}

func Success(format string, args ...interface{}) {
	logMu.Lock()
	defer logMu.Unlock()
	prefix := Green + "[DONE]" + Reset
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s %s\n", ts(), prefix, msg)
}

func Debug(format string, args ...interface{}) {
	if !debugEnabled {
		return
	}
	logMu.Lock()
	defer logMu.Unlock()
	prefix := Gray + "[DBUG]" + Reset
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s %s\n", ts(), prefix, msg)
}