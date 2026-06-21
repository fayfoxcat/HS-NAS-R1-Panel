package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"hs-nas-r1-panel/internal"
)

//go:embed frontend/*
var frontendFS embed.FS

const portFile = "/tmp/r1-panel.port"

func main() {
	port := flag.Int("p", 0, "Web server port (0=random loopback)")
	flag.Parse()

	cmd := "start"
	args := flag.Args()
	if len(args) > 0 {
		cmd = strings.ToLower(args[0])
	}

	switch cmd {
	case "start", "run":
		startWeb(*port)
	case "stop":
		stopService()
		fmt.Println("Stopped.")
	case "install":
		installService(*port)
		if *port > 0 {
			fmt.Printf("Service installed (port %d).\n", *port)
		} else {
			fmt.Println("Service installed (random port, loopback).")
		}
		fmt.Println("Enable: systemctl enable hs-nas-r1-panel")
	case "uninstall":
		uninstallService()
		fmt.Println("Service removed.")
	default:
		fmt.Fprintf(os.Stderr, "Usage: %s [start|stop|install|uninstall] [-p PORT]\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
		os.Exit(1)
	}
}

func startWeb(port int) {
	frontend, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", internal.HandleStatus)
	mux.HandleFunc("/api/reboot", internal.HandleReboot)
	mux.HandleFunc("/api/shutdown", internal.HandleShutdown)
	mux.HandleFunc("/api/screen/off", internal.HandleScreenOff)
	mux.HandleFunc("/api/screen/on", internal.HandleScreenOn)
	mux.Handle("/", http.FileServer(http.FS(frontend)))

	p := port
	if p == 0 {
		p = 40000 + rand.Intn(25535)
	}
	addr := fmt.Sprintf("127.0.0.1:%d", p)
	if port > 0 {
		addr = fmt.Sprintf("0.0.0.0:%d", p)
	}
	log.Printf("r1-panel starting on %s", addr)
	if port == 0 {
		log.Printf("Screen: cog -P drm http://localhost:%d", p)
	}
	log.Printf("Run in background: r1-panel &")
	os.WriteFile(portFile, []byte(strconv.Itoa(p)), 0644)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

const serviceUnit = `[Unit]
Description=HS-NAS-R1 Panel
After=network.target

[Service]
Type=simple
ExecStart=%s%s
ExecStartPost=%s
ExecStop=/usr/bin/pkill cog ; /usr/bin/pkill r1-panel
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`

func installService(port int) {
	exe, _ := os.Executable()
	args := ""
	post := ""
	if port > 0 {
		args = fmt.Sprintf(" -p %d", port)
		post = fmt.Sprintf("/bin/sh -c 'sleep 1; /usr/bin/pkill cog; setsid cog -P drm http://localhost:%d &'", port)
	} else {
		post = fmt.Sprintf("/bin/sh -c 'sleep 2; PORT=$(cat %s); /usr/bin/pkill cog; setsid cog -P drm http://localhost:$PORT &'", portFile)
	}
	unit := fmt.Sprintf(serviceUnit, exe, args, post)
	path := "/etc/systemd/system/hs-nas-r1-panel.service"
	if err := os.WriteFile(path, []byte(unit), 0644); err != nil {
		log.Fatalf("Failed to write service file: %v", err)
	}
	exec.Command("/usr/bin/systemctl", "daemon-reload").Run()
}

func stopService() {
	exec.Command("/usr/bin/systemctl", "stop", "hs-nas-r1-panel").Run()
	exec.Command("/usr/bin/pkill", "cog").Run()
	exec.Command("/usr/bin/pkill", "r1-panel").Run()
}

func uninstallService() {
	stopService()
	exec.Command("/usr/bin/systemctl", "disable", "hs-nas-r1-panel").Run()
	os.Remove("/etc/systemd/system/hs-nas-r1-panel.service")
	exec.Command("/usr/bin/systemctl", "daemon-reload").Run()
	os.Remove(portFile)
}
