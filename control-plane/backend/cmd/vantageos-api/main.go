package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"vantageos.local/control-plane/backend/internal/api"
	"vantageos.local/control-plane/backend/internal/store"
)

func main() {
	statePath := os.Getenv("VANTAGEOS_STATE_PATH")
	st := store.New(statePath)
	if err := st.Initialize(); err != nil {
		log.Fatal(err)
	}

	handler := api.NewHandler(st)
	// Set config root (empty == use absolute paths, fine on target)
	handler.SetConfigRoot("")

	// Regenerate runtime configs if operational
	if err := handler.RegenerateRuntimeConfigs(); err != nil {
		log.Printf("runtime config regeneration: %v", err)
	}

	// Start background pollers (Suricata stats + traffic stats)
	handler.StartBackgroundPollers()

	srv := &http.Server{
		Addr:              "127.0.0.1:5000",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("listening on %s", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}
