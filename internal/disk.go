//go:build linux

package internal

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var diskCache = struct {
	data []map[string]interface{}
	ts   time.Time
}{}

func getStorage() []map[string]interface{} {
	var result []map[string]interface{}
	mounts, _ := os.ReadFile("/proc/mounts")
	for _, line := range strings.Split(string(mounts), "\n") {
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		dev := parts[0]
		mp := parts[1]
		fstype := parts[2]
		// Skip pseudo filesystems
		if fstype == "proc" || fstype == "sysfs" || fstype == "devtmpfs" ||
			fstype == "devpts" || fstype == "tmpfs" || fstype == "cgroup" ||
			fstype == "cgroup2" || fstype == "pstore" || fstype == "bpf" ||
			fstype == "securityfs" || fstype == "debugfs" || fstype == "tracefs" ||
			fstype == "hugetlbfs" || fstype == "mqueue" || fstype == "configfs" ||
			fstype == "fusectl" || fstype == "cgroup2" || fstype == "ramfs" {
			continue
		}

		var stat syscall.Statfs_t
		if err := syscall.Statfs(mp, &stat); err != nil {
			continue
		}
		total := float64(stat.Blocks) * float64(stat.Bsize)
		avail := float64(stat.Bavail) * float64(stat.Bsize)
		free := float64(stat.Bfree) * float64(stat.Bsize)
		used := total - free
		percent := 0.0
		if total > 0 {
			percent = round1(used / total * 100)
		}

		result = append(result, map[string]interface{}{
			"device":   dev,
			"mount":    mp,
			"fstype":   fstype,
			"total_gb": round1(total / gb),
			"used_gb":  round1(used / gb),
			"free_gb":  round1(avail / gb),
			"percent":  percent,
		})
	}
	return result
}

func getDiskHealth() []map[string]interface{} {
	if diskCache.data != nil && time.Since(diskCache.ts) < 60*time.Second {
		return diskCache.data
	}

	disks := make([]map[string]interface{}, 0)

	// Build mount→disk mapping from lsblk JSON tree
	mountDiskMap := buildMountDiskMap()

	// Find root disk (system)
	systemDev := ""
	if out := runCmd("/usr/bin/findmnt", "-n", "-o", "SOURCE", "/"); out != "" {
		systemDev = strings.TrimPrefix(out, "/dev/")
		systemDev = strings.TrimRight(systemDev, "0123456789p")
	}

	// Get physical disks
	out := runCmd("/usr/bin/lsblk", "-dn", "-o", "NAME,SIZE,TYPE,MODEL")
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Fields(line) // handles variable whitespace
		if len(parts) < 3 || parts[2] != "disk" {
			continue
		}
		name := parts[0]
		size := parts[1]
		model := name
		if len(parts) > 3 {
			model = strings.Join(parts[3:], " ")
		}

		role := "other"
		diskType := "disk"
		switch {
		case systemDev != "" && strings.HasPrefix(name, systemDev):
			role = "system"
			if strings.HasPrefix(name, "mmc") {
				diskType = "emmc"
			}
		case strings.HasPrefix(name, "nvme"):
			role = "ssd"
			diskType = "nvme"
		case strings.HasPrefix(name, "sd"):
			role = "hdd"
		case strings.HasPrefix(name, "mmc"):
			diskType = "emmc"
		}

		disk := map[string]interface{}{
			"name":           name,
			"device":         "/dev/" + name,
			"size":           size,
			"model":          model,
			"health":         nil,
			"temperature":    nil,
			"power_on_hours": nil,
			"percent_used":   nil,
			"type":           diskType,
			"role":           role,
			"mounts":         []map[string]interface{}{},
		}

		// Attach mount usage for this disk
		var mounts []map[string]interface{}
		seen := map[string]bool{}
		for mp, dnames := range mountDiskMap {
			for _, dn := range dnames {
				if dn == name && !seen[mp] {
					seen[mp] = true
					var stat syscall.Statfs_t
					if err := syscall.Statfs(mp, &stat); err == nil {
						total := float64(stat.Blocks) * float64(stat.Bsize)
						avail := float64(stat.Bavail) * float64(stat.Bsize)
						free := float64(stat.Bfree) * float64(stat.Bsize)
						used := total - free
						percent := 0.0
						if total > 0 {
							percent = round1(used / total * 100)
						}
						mounts = append(mounts, map[string]interface{}{
							"mount":    mp,
							"total_gb": round1(total / gb),
							"used_gb":  round1(used / gb),
							"free_gb":  round1(avail / gb),
							"percent":  percent,
						})
					}
					break
				}
			}
		}
		disk["mounts"] = mounts

		// eMMC wear (sysfs, no SMART)
		if strings.HasPrefix(name, "mmc") {
			readEMMCWear(name, disk)
		}

		// SMART data (NVMe/SATA)
		smartOut := runCmd("/usr/sbin/smartctl", "-H", "-A", "/dev/"+name)
		parseSMART(smartOut, disk)

		disks = append(disks, disk)
	}

	// Sort: system → ssd → hdd → other
	roleOrder := map[string]int{"system": 0, "ssd": 1, "hdd": 2, "other": 3}
	for i := 0; i < len(disks); i++ {
		for j := i + 1; j < len(disks); j++ {
			if roleOrder[disks[i]["role"].(string)] > roleOrder[disks[j]["role"].(string)] {
				disks[i], disks[j] = disks[j], disks[i]
			}
		}
	}

	diskCache.data = disks
	diskCache.ts = time.Now()
	return disks
}

func buildMountDiskMap() map[string][]string {
	result := map[string][]string{}
	out := runCmd("/usr/bin/lsblk", "-J", "-o", "NAME,SIZE,TYPE,MOUNTPOINT")
	if out == "" {
		return result
	}

	var tree struct {
		Blockdevices []lsblkDevice `json:"blockdevices"`
	}
	if err := json.Unmarshal([]byte(out), &tree); err != nil {
		return result
	}

	var walk func(d lsblkDevice, diskName string)
	walk = func(d lsblkDevice, diskName string) {
		curName := d.Name
		if d.Type == "disk" {
			diskName = curName
		}
		if d.Mountpoint != "" && diskName != "" {
			result[d.Mountpoint] = append(result[d.Mountpoint], diskName)
		}
		for _, c := range d.Children {
			walk(c, diskName)
		}
	}
	for _, d := range tree.Blockdevices {
		walk(d, "")
	}
	return result
}

type lsblkDevice struct {
	Name       string        `json:"name"`
	Type       string        `json:"type"`
	Mountpoint string        `json:"mountpoint"`
	Children   []lsblkDevice `json:"children"`
}

// readEMMCWear reads eMMC life estimation from sysfs (JEDEC eMMC 5.1 spec).
func readEMMCWear(name string, disk map[string]interface{}) {
	base := "/sys/block/" + name + "/device/"

	// life_time: "0x01 0x02" = SLC_used MLC_used (0x01=0-10%, ..., 0x0B=100%+)
	if data, err := os.ReadFile(base + "life_time"); err == nil {
		parts := strings.Fields(string(data))
		for _, val := range parts {
			if v, err := strconv.ParseUint(val, 0, 8); err == nil && v >= 1 && v <= 11 {
				w := float64(v-1) * 10.0
				if disk["percent_used"] == nil || w > disk["percent_used"].(float64) {
					disk["percent_used"] = w
				}
			}
		}
	}

	// pre_eol_info: 0x00=normal, 0x01=warning(80%), 0x02=critical(90%)
	if data, err := os.ReadFile(base + "pre_eol_info"); err == nil {
		val := strings.TrimSpace(string(data))
		if val == "0x02" {
			disk["health"] = "FAILED"
		} else if val == "0x01" {
			// Warning but not failed yet
			if disk["health"] == nil {
				disk["health"] = "PASSED"
			}
		}
	}
}

func parseSMART(out string, disk map[string]interface{}) {
	if out == "" {
		return
	}
	for _, line := range strings.Split(out, "\n") {
		low := strings.ToLower(line)
		switch {
		case strings.Contains(low, "overall-health"):
			if strings.Contains(low, "passed") {
				disk["health"] = "PASSED"
			} else if strings.Contains(low, "failed") {
				disk["health"] = "FAILED"
			}
		case strings.HasPrefix(low, "temperature:"):
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				if t, err := strconv.Atoi(parts[1]); err == nil {
					disk["temperature"] = t
				}
			}
		case strings.Contains(low, "percentage used:"):
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				val := strings.TrimSpace(parts[1])
				val = strings.TrimSuffix(val, "%")
				if f, err := strconv.ParseFloat(val, 64); err == nil {
					disk["percent_used"] = f
				}
			}
		case strings.HasPrefix(low, "power on hours:"):
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				val := strings.TrimSpace(parts[1])
				val = strings.ReplaceAll(val, ",", "")
				if h, err := strconv.ParseFloat(val, 64); err == nil {
					disk["power_on_hours"] = h
				}
			}
		case strings.Contains(low, "temperature_celsius") || strings.Contains(low, "airflow_temperature_cel"):
			parts := strings.Fields(line)
			if len(parts) >= 10 && disk["temperature"] == nil {
				if t, err := strconv.Atoi(parts[9]); err == nil {
					disk["temperature"] = t
				}
			}
		case strings.Contains(low, "power_on_hours"):
			parts := strings.Fields(line)
			if len(parts) >= 10 {
				if h, err := strconv.Atoi(parts[9]); err == nil {
					disk["power_on_hours"] = h
				}
			}
		}
	}
}
