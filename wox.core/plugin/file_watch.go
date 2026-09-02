package plugin

import (
	"os"
	"time"

	"wox/util"
)

func debounceKeyedTimer(timers *util.HashMap[string, *time.Timer], key string, delay time.Duration, fn func()) {
	if timer, exists := timers.Load(key); exists {
		timer.Stop()
	}
	var timer *time.Timer
	timer = time.AfterFunc(delay, func() {
		current, exists := timers.Load(key)
		if !exists || current != timer {
			return
		}
		timers.Delete(key)
		fn()
	})
	timers.Store(key, timer)
}

func readFileWithRetry(filePath string, retryWindow time.Duration) ([]byte, error) {
	deadline := time.Now().Add(retryWindow)
	var lastData []byte
	var lastErr error
	for {
		data, err := os.ReadFile(filePath)
		if err != nil {
			lastErr = err
		} else if lastData != nil && len(data) == len(lastData) {
			return data, nil
		} else {
			lastData = data
			lastErr = nil
		}
		if !time.Now().Before(deadline) {
			if lastData != nil {
				return lastData, nil
			}
			return nil, lastErr
		}
		time.Sleep(50 * time.Millisecond)
	}
}
