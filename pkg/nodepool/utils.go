package nodepool

import (
	"fmt"
	"time"
)

const (
	dateTimeFormat = "2006-01-02T15:04"
)

func NodepoolName() string {
	return fmt.Sprintf("%s-%s", NODEPOOL_PREFIX, time.Now().Format(dateTimeFormat))
}
