//go:build linux

package main

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

var (
	hwOnce  sync.Once
	hwModel string
)

// hardwareModel identifies the board using the kernel's CPU description
// ("AISLPC R36TMax" on RK3326 handhelds). It is cached because neither
// /proc/cpuinfo nor the discovery loop changes at runtime.
func hardwareModel() string {
	hwOnce.Do(func() {
		hwModel = readSysInfo("").CPUModel
	})
	if hwModel == "" {
		return "rk3326"
	}
	return hwModel
}

func readSysInfo(romsRoot string) SysInfo {
	si := SysInfo{Cores: runtime.NumCPU()}

	if b, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		sc := bufio.NewScanner(strings.NewReader(string(b)))
		for sc.Scan() {
			parts := strings.SplitN(sc.Text(), ":", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(parts[0]))
			val := strings.TrimSpace(parts[1])
			if val == "" {
				continue
			}
			switch key {
			case "hardware":
				si.CPUModel = val
			case "model name":
				if si.CPUModel == "" {
					si.CPUModel = val
				}
			case "processor":
				// ARM kernels expose the core model here; keep the first one.
				if si.CPUModel == "" {
					si.CPUModel = val
				}
			}
		}
	}

	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		f := strings.Fields(string(b))
		if len(f) > 0 {
			si.Load1, _ = strconv.ParseFloat(f[0], 64)
		}
	}

	if b, err := os.ReadFile("/proc/meminfo"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.Fields(line)
			if len(f) < 2 {
				continue
			}
			n, _ := strconv.ParseInt(f[1], 10, 64)
			switch f[0] {
			case "MemTotal:":
				si.MemTotalMB = n / 1024
			case "MemAvailable:":
				si.MemAvailMB = n / 1024
			}
		}
	}

	if romsRoot != "" {
		var st syscall.Statfs_t
		if err := syscall.Statfs(romsRoot, &st); err == nil {
			total := float64(st.Blocks) * float64(st.Bsize)
			free := float64(st.Bavail) * float64(st.Bsize)
			si.DiskTotalGB = round1(total / (1 << 30))
			si.DiskFreeGB = round1(free / (1 << 30))
		}
	}

	si.TempC = readMaxTemp()

	if b, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		si.Kernel = strings.TrimSpace(string(b))
	}
	if b, err := os.ReadFile("/proc/uptime"); err == nil {
		f := strings.Fields(string(b))
		if len(f) > 0 {
			v, _ := strconv.ParseFloat(f[0], 64)
			si.UptimeSec = int64(v)
		}
	}
	return si
}

func readMaxTemp() float64 {
	paths, _ := filepath.Glob("/sys/class/thermal/thermal_zone*/temp")
	var max float64
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
		if err != nil {
			continue
		}
		if v > 1000 { // millidegrees
			v /= 1000
		}
		if v > max && v < 150 {
			max = v
		}
	}
	return round1(max)
}
