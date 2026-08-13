package logger

import (
	"sync"
	"testing"
)

// TestConcurrentLoggingNoRace exercises Println/LogUI from many goroutines
// at once. Run with -race to catch a regression of the previously
// unsynchronized LogList/LogTrayList globals.
func TestConcurrentLoggingNoRace(t *testing.T) {
	Init()
	// Drain the channel LogTray writes to so it never blocks the writers.
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-GetLogTrayChan():
			case <-done:
				return
			}
		}
	}()
	defer close(done)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			Println("log line", n)
		}(i)
		go func(n int) {
			defer wg.Done()
			LogUI("ui line", n)
		}(i)
	}
	wg.Wait()

	_ = GetLogList()
	_ = GetLogTrayList()
}
