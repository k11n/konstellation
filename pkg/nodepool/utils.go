package nodepool

import (
	"fmt"
	"time"
)

const (
	dateTimeFormat = "20060102-1504"
)

func NodepoolName() string {
	return fmt.Sprintf("%s-%s", NODEPOOL_PREFIX, time.Now().Format(dateTimeFormat))
}
