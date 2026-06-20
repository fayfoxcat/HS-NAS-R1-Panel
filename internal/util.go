package internal

import (
	"os"
	"os/exec"
	"strings"
	"sync"
)

var hostOnce sync.Once
var host string

func hostname() string {
	hostOnce.Do(func() {
		h, err := os.Hostname()
		if err != nil {
			host = "unknown"
			return
		}
		host = h
	})
	return host
}

func runCmd(cmd string, args ...string) string {
	ctx := exec.Command(cmd, args...)
	ctx.Stderr = nil
	out, err := ctx.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func osWriteFile(path, data string) error {
	return os.WriteFile(path, []byte(data), 0644)
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}
