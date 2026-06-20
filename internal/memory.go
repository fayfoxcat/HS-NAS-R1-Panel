package internal

import (
	"os"
	"strconv"
	"strings"
)

func getMemory() map[string]interface{} {
	mem := readMeminfo()
	total := mem["MemTotal"]
	avail := mem["MemAvailable"]
	if avail == 0 {
		avail = mem["MemFree"] + mem["Buffers"] + mem["Cached"]
	}
	used := total - avail
	percent := 0.0
	if total > 0 {
		percent = round1(float64(used) / float64(total) * 100)
	}

	kbToGB := func(kb int64) float64 {
		return round1(float64(kb) / (1024.0 * 1024.0))
	}

	swapTotal := mem["SwapTotal"]
	swapFree := mem["SwapFree"]
	swapUsed := swapTotal - swapFree
	swapPercent := 0.0
	if swapTotal > 0 {
		swapPercent = round1(float64(swapUsed) / float64(swapTotal) * 100)
	}

	return map[string]interface{}{
		"total_gb":     kbToGB(total),
		"used_gb":      kbToGB(used),
		"available_gb": kbToGB(avail),
		"percent":      percent,
		"swap_total_gb": kbToGB(swapTotal),
		"swap_used_gb":  kbToGB(swapUsed),
		"swap_percent":  swapPercent,
	}
}

const gb = 1024.0 * 1024.0 * 1024.0

func readMeminfo() map[string]int64 {
	result := map[string]int64{}
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return result
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.TrimSuffix(val, " kB")
		val = strings.TrimSpace(val)
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			continue
		}
		result[key] = n
	}
	return result
}
