package utils

import (
	"context"
	"time"
)

const (
	MediumCheckInterval  = 1000    // 1s
	LongCheckInterval    = 5000    // 5s
	ShortTimeoutSec      = 3 * 60  // 3 mins
	MediumTimeoutSec     = 6 * 60  // 6 mins
	LongTimeoutSec       = 15 * 60 // 15 mins
	ReallyLongTimeoutSec = 30 * 60 // 30 mins
)

func WaitUntilComplete(timeoutSec int, checkInterval int, checkFunc func() (bool, error)) error {
	timeout := time.After(time.Second * time.Duration(timeoutSec))

	for {
		success, err := checkFunc()
		rest := time.After(time.Millisecond * time.Duration(checkInterval))
		if err != nil {
			return err
		}
		if success {
			break
		}
		select {
		case <-timeout:
			return context.DeadlineExceeded
		case <-rest:
			continue
		}
	}
	return nil
}

func Retry(retryFunc func() error, numTimes int, backoff int) error {
	if backoff == 0 {
		backoff = MediumCheckInterval
	}
	delay := backoff
	var lastErr error
	for i := 0; i < numTimes; i++ {
		if lastErr = retryFunc(); lastErr == nil {
			return nil
		}
		time.Sleep(time.Duration(delay) * time.Millisecond)
		delay += backoff
	}
	return lastErr
}
