package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"
)

func main() {
	switch os.Getenv("TEST_CASE") {
	case "OOM":
		oom := make(map[int]float64)
		for i := 0; ; i++ {
			oom[i] = rand.Float64()
		}
	case "PANIC":
		time.Sleep(time.Second * 10)
		panic("at the disco")
	default:
		for {
			fmt.Println("Taking a nap")
			time.Sleep(time.Second * 60)
		}
	}
}
