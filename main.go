package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"hs-nas-r1-panel/internal"
)

//go:embed frontend/*
var frontendFS embed.FS

func main() {
	port := "8088"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	frontend, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/status", internal.HandleStatus)
	mux.HandleFunc("/api/reboot", internal.HandleReboot)
	mux.HandleFunc("/api/shutdown", internal.HandleShutdown)
	mux.HandleFunc("/api/screen/off", internal.HandleScreenOff)
	mux.HandleFunc("/api/screen/on", internal.HandleScreenOn)

	// Static frontend
	mux.Handle("/", http.FileServer(http.FS(frontend)))

	log.Printf("NAS Panel starting on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
