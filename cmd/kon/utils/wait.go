package utils

import (
	"context"
	"time"
)

const (
	MediumCheckInterval = 1000 // 1s
	LongCheckInterval   = 5000 // 5s
	LongTimeoutSec      = 600  // 10 mins
	ShortTimeoutSec     = 60   // 1 mins
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
