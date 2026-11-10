//go:build !linux

package main

// The daemon is built for Linux consoles; this stub keeps the Windows test
// build working.
func readSysInfo(romsRoot string) SysInfo {
	return SysInfo{Cores: 0}
}

func hardwareModel() string {
	return "rk3326"
}
