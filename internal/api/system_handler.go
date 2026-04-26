package api

import (
	"io/ioutil"
	"net/http"
	"runtime"
	"strconv"
	"strings"
)

var (
	lastSysTotal uint64
	lastSysIdle  uint64
	lastAppTime  uint64
	lastAppTotal uint64
)

func (h *APIHandler) GetSystemStats(w http.ResponseWriter, r *http.Request) {
	// 1. App RAM
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	appRamBytes := mem.Alloc

	// 2. Global RAM
	var totalRam, freeRam, buffers, cached uint64
	meminfo, err := ioutil.ReadFile("/proc/meminfo")
	if err == nil {
		lines := strings.Split(string(meminfo), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			val, _ := strconv.ParseUint(fields[1], 10, 64)
			val *= 1024 // kb to bytes
			switch fields[0] {
			case "MemTotal:":
				totalRam = val
			case "MemFree:":
				freeRam = val
			case "Buffers:":
				buffers = val
			case "Cached:":
				cached = val
			}
		}
	}
	usedRam := totalRam - freeRam - buffers - cached
	if totalRam == 0 {
		totalRam = 1 // avoid div by zero
	}

	// 3. Global CPU
	var sysTotal, sysIdle uint64
	stat, err := ioutil.ReadFile("/proc/stat")
	if err == nil {
		lines := strings.Split(string(stat), "\n")
		if len(lines) > 0 {
			fields := strings.Fields(lines[0])
			if len(fields) > 4 && fields[0] == "cpu" {
				for i := 1; i < len(fields); i++ {
					v, _ := strconv.ParseUint(fields[i], 10, 64)
					sysTotal += v
					if i == 4 { // idle is the 4th field after 'cpu' (fields[4])
						sysIdle = v
					}
				}
			}
		}
	}

	sysCpuPercent := 0.0
	if lastSysTotal > 0 && sysTotal > lastSysTotal {
		totalDiff := sysTotal - lastSysTotal
		idleDiff := sysIdle - lastSysIdle
		sysCpuPercent = 100.0 * float64(totalDiff-idleDiff) / float64(totalDiff)
	}
	lastSysTotal = sysTotal
	lastSysIdle = sysIdle

	// 4. App CPU
	var appTime uint64
	selfStat, err := ioutil.ReadFile("/proc/self/stat")
	if err == nil {
		fields := strings.Fields(string(selfStat))
		if len(fields) >= 15 {
			utime, _ := strconv.ParseUint(fields[13], 10, 64)
			stime, _ := strconv.ParseUint(fields[14], 10, 64)
			appTime = utime + stime
		}
	}

	appCpuPercent := 0.0
	if lastAppTotal > 0 && sysTotal > lastAppTotal {
		totalDiff := sysTotal - lastAppTotal
		appDiff := appTime - lastAppTime
		appCpuPercent = 100.0 * float64(appDiff) / float64(totalDiff)
		appCpuPercent *= float64(runtime.NumCPU())
	}
	lastAppTotal = sysTotal
	lastAppTime = appTime

	jsonResponse(w, map[string]interface{}{
		"app_ram_mb":      float64(appRamBytes) / 1024 / 1024,
		"sys_ram_mb":      float64(usedRam) / 1024 / 1024,
		"total_ram_mb":    float64(totalRam) / 1024 / 1024,
		"sys_ram_percent": 100.0 * float64(usedRam) / float64(totalRam),
		"sys_cpu_percent": sysCpuPercent,
		"app_cpu_percent": appCpuPercent,
		"num_goroutine":   runtime.NumGoroutine(),
	})
}
