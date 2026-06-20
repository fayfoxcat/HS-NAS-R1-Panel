package internal

import (
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	prevNetData  map[string][2]int64 // rx, tx bytes
	prevNetTime  time.Time
	netMu        sync.Mutex
)

func getNetwork() []map[string]interface{} {
	netMu.Lock()
	defer netMu.Unlock()

	curr := readNetDev()
	currTime := time.Now()
	elapsed := currTime.Sub(prevNetTime).Seconds()
	if elapsed < 0.5 {
		elapsed = 1
	}

	ifaces, _ := net.Interfaces()
	addrMap := map[string]net.Interface{}
	for _, iface := range ifaces {
		addrMap[iface.Name] = iface
	}

	var result []map[string]interface{}
	for name, stats := range curr {
		ipv4 := []string{}
		ipv6 := []string{}
		isUp := false

		if iface, ok := addrMap[name]; ok {
			isUp = iface.Flags&net.FlagUp != 0
			addrs, _ := iface.Addrs()
			for _, a := range addrs {
				ipnet, ok := a.(*net.IPNet)
				if !ok {
					continue
				}
				if ip4 := ipnet.IP.To4(); ip4 != nil {
					ipv4 = append(ipv4, ip4.String())
				} else if !strings.HasPrefix(ipnet.IP.String(), "fe80") {
					ipv6 = append(ipv6, ipnet.IP.String())
				}
			}
		}

		rxSpeed := 0.0
		txSpeed := 0.0
		if prev, ok := prevNetData[name]; ok {
			rxSpeed = float64(stats[0]-prev[0]) / elapsed
			txSpeed = float64(stats[1]-prev[1]) / elapsed
		}

		result = append(result, map[string]interface{}{
			"name":           name,
			"is_up":          isUp,
			"speed_mbps":     0,
			"rx_bytes":       stats[0],
			"tx_bytes":       stats[1],
			"rx_speed_bytes": round1(rxSpeed),
			"tx_speed_bytes": round1(txSpeed),
			"ipv4":           ipv4,
			"ipv6":           ipv6,
		})
	}

	prevNetData = curr
	prevNetTime = currTime
	return result
}

// readNetDev parses /proc/net/dev, returns map[iface]→[rx_bytes, tx_bytes]
func readNetDev() map[string][2]int64 {
	result := map[string][2]int64{}
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return result
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		if len(fields) < 9 {
			continue
		}
		rx, _ := strconv.ParseInt(fields[0], 10, 64)
		tx, _ := strconv.ParseInt(fields[8], 10, 64)
		result[name] = [2]int64{rx, tx}
	}
	return result
}
