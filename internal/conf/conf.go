package conf

import "time"

type FilteredResult struct {
	Count int64       `json:"count"`
	Total int64       `json:"total"`
	Items interface{} `json:"items"`
}

type PerfConfig struct {
	Frequency int
	Duration  time.Duration
	Node      string
	Recipient string
}
