package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"hs-nas-r1-panel/internal"
)

//go:embed frontend/*
var frontendFS embed.FS

func main() {
	web := flag.Bool("web", false, "Start web server")
	port := flag.String("port", "8088", "Web server port")
	install := flag.Bool("install", false, "Install systemd auto-start service")
	uninstall := flag.Bool("uninstall", false, "Remove systemd auto-start service")
	flag.Parse()

	switch {
	case *install:
		installService()
		fmt.Println("Systemd service installed. Enable with: systemctl enable hs-nas-r1-panel")
	case *uninstall:
		uninstallService()
		fmt.Println("Systemd service removed.")
	case *web:
		startWeb(*port)
	default:
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
		os.Exit(1)
	}
}

func startWeb(port string) {
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

	log.Printf("HS-NAS-R1 Panel starting on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

const serviceUnit = `[Unit]
Description=HS-NAS-R1 Panel
After=network.target

[Service]
Type=simple
ExecStart=%s --web --port %s
ExecStartPost=/bin/sh -c "sleep 2; pkill cog 2>/dev/null; setsid cog -P drm http://localhost:8088"
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`

func installService() {
	exe, _ := os.Executable()
	unit := fmt.Sprintf(serviceUnit, exe, "8088")
	path := "/etc/systemd/system/hs-nas-r1-panel.service"
	if err := os.WriteFile(path, []byte(unit), 0644); err != nil {
		log.Fatalf("Failed to write service file: %v", err)
	}
	exec.Command("/usr/bin/systemctl", "daemon-reload").Run()
}

func uninstallService() {
	exec.Command("/usr/bin/systemctl", "stop", "hs-nas-r1-panel").Run()
	exec.Command("/usr/bin/systemctl", "disable", "hs-nas-r1-panel").Run()
	os.Remove("/etc/systemd/system/hs-nas-r1-panel.service")
	exec.Command("/usr/bin/systemctl", "daemon-reload").Run()
}
