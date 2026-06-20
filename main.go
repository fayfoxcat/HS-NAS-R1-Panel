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

	any := false

	if *uninstall {
		uninstallService()
		fmt.Println("Systemd service removed.")
		any = true
	}

	if *install {
		installService(*web, *port)
		fmt.Printf("Systemd service installed (--web --port %s).\n", *port)
		fmt.Println("Enable with: systemctl enable hs-nas-r1-panel")
		any = true
	}

	if *web {
		startWeb(*port)
		any = true
	}

	if !any {
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
	mux.HandleFunc("/api/exit", handleExit)
	mux.HandleFunc("/api/screen/off", internal.HandleScreenOff)
	mux.HandleFunc("/api/screen/on", internal.HandleScreenOn)
	mux.Handle("/", http.FileServer(http.FS(frontend)))

	log.Printf("HS-NAS-R1 Panel starting on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

func handleExit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"exiting"}`))
	go func() {
		exec.Command("/usr/bin/pkill", "cog").Run()
	}()
}

const serviceUnit = `[Unit]
Description=HS-NAS-R1 Panel
After=network.target

[Service]
Type=simple
ExecStart=%s%s
ExecStop=/usr/bin/pkill hs-nas-r1-panel ; /usr/bin/pkill cog
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`

func installService(web bool, port string) {
	exe, _ := os.Executable()
	args := fmt.Sprintf(" --web --port %s", port) // always include web
	unit := fmt.Sprintf(serviceUnit, exe, args)
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
	exec.Command("/usr/bin/pkill", "hs-nas-r1-panel").Run()
	exec.Command("/usr/bin/pkill", "cog").Run()
}
