package logger

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

var debugLogging bool = true

var logMu sync.Mutex
var LogList string
var LogArray []string

var logTrayMu sync.Mutex
var LogTrayList string
var LogTrayArray []string
var LogTrayChan chan string

func GetLogTrayChan() chan string {
	return LogTrayChan
}

// GetLogList returns a snapshot of LogList safe for concurrent reads.
func GetLogList() string {
	logMu.Lock()
	defer logMu.Unlock()
	return LogList
}

// GetLogTrayList returns a snapshot of LogTrayList safe for concurrent reads.
func GetLogTrayList() string {
	logTrayMu.Lock()
	defer logTrayMu.Unlock()
	return LogTrayList
}

func Init() {
	LogTrayChan = make(chan string)
	if os.Getenv("DEBUG_LOGGING") == "true" {
		debugLogging = true
	} else {
		debugLogging = false
	}
}

func Println(a ...any) {
	LogTray(a...)
	if debugLogging {
		fmt.Println(a...)
	}
}

func LogUI(a ...any) {
	entry := time.Now().Format("2006.01.02 15:04:05") + ": " + fmt.Sprint(a...) + "\n"

	logMu.Lock()
	defer logMu.Unlock()
	LogArray = append(LogArray, entry)
	if len(LogArray) >= 50 {
		LogArray = LogArray[1:]
	}
	var b strings.Builder
	for _, e := range LogArray {
		b.WriteString(e)
	}
	LogList = b.String()
}

func LogTray(a ...any) {
	entry := time.Now().Format("2006.01.02 15:04:05") + ": " + fmt.Sprint(a...) + "\n"

	logTrayMu.Lock()
	LogTrayArray = append(LogTrayArray, entry)
	if len(LogTrayArray) >= 200 {
		LogTrayArray = LogTrayArray[1:]
	}
	var b strings.Builder
	for _, e := range LogTrayArray {
		b.WriteString(e)
	}
	LogTrayList = b.String()
	logTrayMu.Unlock()

	select {
	case LogTrayChan <- entry:
	default:
	}
}
