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

	"hs-nas-r1-panel/internal"
)

//go:embed frontend/*
var frontendFS embed.FS

const portFile = "/tmp/r1-panel.port"

func main() {
	port := flag.Int("p", 0, "Web server port (0=random loopback)")
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
		installService(*port)
		fmt.Printf("Systemd service installed.")
		if *port > 0 {
			fmt.Printf(" (port %d)\n", *port)
		} else {
			fmt.Println(" (random port, loopback only)")
		}
		fmt.Println("Enable with: systemctl enable hs-nas-r1-panel")
		any = true
	}

	if !*install {
		startWeb(*port)
		any = true
	}

	if !any {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n", filepath.Base(os.Args[0]))
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
	mux.HandleFunc("/api/exit", handleExit)
	mux.HandleFunc("/api/screen/off", internal.HandleScreenOff)
	mux.HandleFunc("/api/screen/on", internal.HandleScreenOn)
	mux.Handle("/", http.FileServer(http.FS(frontend)))

	p := resolvePort(port)
	addr := fmt.Sprintf("0.0.0.0:%d", p)
	if port == 0 {
		addr = fmt.Sprintf("127.0.0.1:%d", p)
	}
	log.Printf("r1-panel starting on %s", addr)
	os.WriteFile(portFile, []byte(strconv.Itoa(p)), 0644)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func resolvePort(port int) int {
	if port > 0 {
		return port
	}
	return 40000 + rand.Intn(25535) // 40000-65535
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
ExecStartPost=%s
ExecStop=/usr/bin/pkill cog ; /usr/bin/pkill hs-nas-r1-panel
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
		post = fmt.Sprintf("/bin/sh -c 'sleep 1; /usr/bin/pkill cog; cog -P drm http://localhost:%d'", port)
	} else {
		// Random port: ExecStartPost reads the port file and launches cog
		post = fmt.Sprintf("/bin/sh -c 'sleep 2; PORT=$(cat %s); /usr/bin/pkill cog; cog -P drm http://localhost:$PORT'", portFile)
	}
	unit := fmt.Sprintf(serviceUnit, exe, args, post)
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
	exec.Command("/usr/bin/pkill", "cog").Run()
	exec.Command("/usr/bin/pkill", "hs-nas-r1-panel").Run()
}
