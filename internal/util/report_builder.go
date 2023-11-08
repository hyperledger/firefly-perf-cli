package util

import (
	"fmt"
	"sync"
	"time"
)

type Summary struct {
	Latency     *Latency `json:"latency"`
	TPS         *TPS     `json:"tps"`
	ResultCount *TPS     `json:"resultCount"`
}

type SystemUnderTest struct {
}

type TPS struct {
	SendRate   float64 `json:"sendRate"`
	Throughput float64 `json:"throughput"`
}

type ResultCount struct {
	TotalCount int64 `json:"totalCount"`
	RetryCount int64 `json:"retryCount"`
}

type Latency struct {
	mux   sync.Mutex
	min   time.Duration
	max   time.Duration
	total int64
	count int64
}

func (lt *Latency) Record(latency time.Duration) {
	lt.mux.Lock()
	defer lt.mux.Unlock()
	if latency < lt.min || lt.min.Nanoseconds() == 0 {
		lt.min = latency
	}
	if latency > lt.max {
		lt.max = latency
	}
	lt.total += latency.Milliseconds()
	lt.count++
}

func (lt *Latency) Avg() time.Duration {
	return time.Duration((lt.total / lt.count) * int64(time.Millisecond))
}

func (lt *Latency) Min() time.Duration {
	return lt.min
}

func (lt *Latency) Max() time.Duration {
	return lt.max
}
func (lt *Latency) String() string {
	return fmt.Sprintf("min: %s, max: %s, avg: %s", lt.Min(), lt.Max(), lt.Avg())
}
