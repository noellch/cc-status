package main

import (
	"log"
	"time"

	"fyne.io/systray"
	"github.com/anthropics/cc-status-go/internal/server"
	"github.com/anthropics/cc-status-go/internal/session"
	"github.com/anthropics/cc-status-go/internal/tray"
	"github.com/anthropics/cc-status-go/pkg/model"
)

func main() {
	store := session.NewStore()
	store.LoadFromDisk()

	srv := server.New(model.SocketPath(), func(e model.SessionEvent) {
		store.HandleEvent(e)
	})
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	// Periodic cleanup every 60s.
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			store.CleanupStale()
		}
	}()

	t := tray.NewTray(store)
	systray.Run(t.OnReady, func() {
		srv.Stop()
		t.OnExit()
	})
}
