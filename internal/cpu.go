package internal

import (
	"os"
	"strconv"
	"strings"
	"time"
)

func getCPU() map[string]interface{} {
	result := map[string]interface{}{
		"percent":      readCPUPercent(),
		"count":        cpuCount(),
		"freq_current": nil,
		"freq_max":     nil,
		"temperature":  readCPUTemp(),
	}

	// CPU frequency from sysfs
	if data, err := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq"); err == nil {
		if f, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
			result["freq_current"] = float64(f) / 1000.0 // kHz to MHz
		}
	}
	return result
}

func cpuCount() int {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return 0
	}
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "processor") {
			count++
		}
	}
	return count
}

// readCPUPercent does a simple /proc/stat poll (500ms interval).
func readCPUPercent() float64 {
	a := readCPUTimes()
	time.Sleep(500 * time.Millisecond)
	b := readCPUTimes()
	if a == nil || b == nil || len(a) < 4 || len(b) < 4 {
		return 0
	}
	aIdle := a[3] + a[4] // idle + iowait
	bIdle := b[3] + b[4]
	aTotal := sum(a)
	bTotal := sum(b)
	totalDelta := bTotal - aTotal
	idleDelta := bIdle - aIdle
	if totalDelta <= 0 {
		return 0
	}
	return round1((1 - idleDelta/totalDelta) * 100)
}

func getGPU() map[string]interface{} {
	info := map[string]interface{}{
		"name":        "Intel UHD Graphics (N100)",
		"load_pct":    nil,
		"temperature": nil,
		"memory":      nil,
	}
	// Try reading GPU frequency from sysfs
	if data, err := os.ReadFile("/sys/class/drm/card1/gt_cur_freq_mhz"); err == nil {
		if f, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
			info["freq_mhz"] = f
		}
	}
	return info
}

func readCPUTimes() []float64 {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		parts := strings.Fields(line)
		var vals []float64
		for _, p := range parts[1:] {
			f, _ := strconv.ParseFloat(p, 64)
			vals = append(vals, f)
		}
		return vals
	}
	return nil
}

func sum(v []float64) float64 {
	var s float64
	for _, x := range v {
		s += x
	}
	return s
}

func readCPUTemp() interface{} {
	// Try hwmon first
	entries, _ := os.ReadDir("/sys/class/hwmon")
	for _, e := range entries {
		prefix := "/sys/class/hwmon/" + e.Name() + "/"
		for i := 1; i <= 5; i++ {
			labelPath := prefix + "temp" + strconv.Itoa(i) + "_label"
			if data, err := os.ReadFile(labelPath); err == nil {
				label := string(data)
				if strings.Contains(label, "CPU") || strings.Contains(label, "Package") || strings.Contains(label, "Core") {
					inputPath := prefix + "temp" + strconv.Itoa(i) + "_input"
					if data, err := os.ReadFile(inputPath); err == nil {
						if t, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
							return round1(float64(t) / 1000.0)
						}
					}
				}
			}
			// Also try without label
			inputPath := prefix + "temp" + strconv.Itoa(i) + "_input"
			if data, err := os.ReadFile(inputPath); err == nil {
				if t, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
					tv := float64(t) / 1000.0
					if tv > 0 && tv < 150 {
						return round1(tv)
					}
				}
			}
		}
	}
	// Fallback: thermal zone
	if data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp"); err == nil {
		if t, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
			return round1(float64(t) / 1000.0)
		}
	}
	return nil
}
