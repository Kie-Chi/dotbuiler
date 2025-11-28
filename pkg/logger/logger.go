package logger

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	debugEnabled bool
	logMu        sync.Mutex
)

// ANSI Color Codes
const (
	Reset   = "\033[0m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	Gray    = "\033[90m"
	White   = "\033[97m"
)

func init() {
	v := strings.ToLower(os.Getenv("DOTBUILDER_DEBUG"))
	debugEnabled = v == "1" || v == "true" || v == "yes"
}

func ts() string {
	return Gray + time.Now().Format("15:04:05") + Reset
}

func printLog(prefix, msg string) {
	logMu.Lock()
	defer logMu.Unlock()
	fmt.Printf("%s %s %s\n", ts(), prefix, msg)
}

func SetDebug(enable bool) { debugEnabled = enable }

func Info(format string, args ...interface{}) {
	printLog(Blue+"[INFO]"+Reset, fmt.Sprintf(format, args...))
}

func Warn(format string, args ...interface{}) {
	printLog(Yellow+"[WARN]"+Reset, fmt.Sprintf(format, args...))
}

func Error(format string, args ...interface{}) {
	printLog(Red+"[ERRO]"+Reset, fmt.Sprintf(format, args...))
	os.Exit(1)
}

func Success(format string, args ...interface{}) {
	printLog(Green+"[DONE]"+Reset, fmt.Sprintf(format, args...))
}

func Debug(format string, args ...interface{}) {
	if !debugEnabled {
		return
	}
	printLog(Gray+"[DBUG]"+Reset, fmt.Sprintf(format, args...))
}


func InfoFile(format string, args ...interface{}) {
	prefix := Magenta + "[FILE]" + Reset
	printLog(prefix, fmt.Sprintf(format, args...))
}

func InfoPkg(format string, args ...interface{}) {
	prefix := Cyan + "[PKG ]" + Reset
	printLog(prefix, fmt.Sprintf(format, args...))
}

func InfoTask(format string, args ...interface{}) {
	prefix := Blue + "[TASK]" + Reset
	printLog(prefix, fmt.Sprintf(format, args...))
}