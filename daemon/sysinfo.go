package main

import "math"

// SysInfo is intentionally tiny and read on demand: /proc reads plus one
// statfs. It is never polled in the background.
type SysInfo struct {
	CPUModel    string  `json:"cpu,omitempty"`
	Cores       int     `json:"cores"`
	Load1       float64 `json:"load1"`
	MemTotalMB  int64   `json:"memTotalMB"`
	MemAvailMB  int64   `json:"memAvailMB"`
	DiskTotalGB float64 `json:"diskTotalGB"`
	DiskFreeGB  float64 `json:"diskFreeGB"`
	TempC       float64 `json:"tempC,omitempty"`
	Kernel      string  `json:"kernel,omitempty"`
	UptimeSec   int64   `json:"uptimeSec"`
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}
