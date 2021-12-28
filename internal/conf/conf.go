package conf

import (
	"time"
)

type FilteredResult struct {
	Count int64       `json:"count"`
	Items interface{} `json:"items"`
	Total int64       `json:"total"`
}

type PerfConfig struct {
	Cmd       string
	Duration  time.Duration
	Frequency int
	Jobs      int
	Node      string
	Recipient string
	Workers   int
}
