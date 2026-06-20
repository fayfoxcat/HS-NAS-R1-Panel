package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func getDocker() []map[string]interface{} {
	out := runCmd("/usr/bin/docker", "ps", "-a", "--format", `{{json .}}`)
	if out == "" {
		return []map[string]interface{}{}
	}
	var result []map[string]interface{}
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			continue
		}
		result = append(result, data)
	}
	return result
}

func getVMs() []map[string]interface{} {
	out := runCmd("/usr/bin/virsh", "list", "--all")
	if out == "" {
		return []map[string]interface{}{}
	}
	var result []map[string]interface{}
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		if i < 2 || line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			result = append(result, map[string]interface{}{
				"id":    parts[0],
				"name":  parts[1],
				"state": strings.Join(parts[2:], " "),
			})
		}
	}
	return result
}

func getServices() []map[string]interface{} {
	svcNames := []string{
		"docker.service", "libvirtd.service", "containerd.service",
		"NetworkManager.service", "cron.service",
	}
	var result []map[string]interface{}
	for _, name := range svcNames {
		out := runCmd("systemctl", "is-active", name)
		result = append(result, map[string]interface{}{
			"name":   strings.TrimSuffix(name, ".service"),
			"active": out == "active",
		})
	}
	return result
}

func getUptime() map[string]interface{} {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return map[string]interface{}{"uptime_str": "--"}
	}
	parts := strings.Fields(string(data))
	if len(parts) < 1 {
		return map[string]interface{}{"uptime_str": "--"}
	}
	secs, _ := strconv.ParseFloat(parts[0], 64)
	d := time.Duration(secs) * time.Second
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	return map[string]interface{}{
		"uptime_str":     fmt.Sprintf("%d天 %d时 %d分", days, hours, minutes),
		"uptime_days":    days,
		"uptime_hours":   hours,
		"uptime_minutes": minutes,
	}
}
