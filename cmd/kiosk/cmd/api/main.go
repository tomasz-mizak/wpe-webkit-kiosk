package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tomasz-mizak/wpe-webkit-kiosk/cmd/kiosk/internal/api"
	"github.com/tomasz-mizak/wpe-webkit-kiosk/cmd/kiosk/internal/config"
)

func main() {
	cfg, err := config.Load(config.DefaultPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	port := cfg.Get("API_PORT")
	if port == "" {
		port = "8100"
	}

	token := cfg.Get("API_TOKEN")
	if token == "" {
		log.Fatal("API_TOKEN is not configured. Run: kiosk api token regenerate")
	}

	srv := api.NewServer(port, token)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		addr := fmt.Sprintf("0.0.0.0:%s", port)
		log.Printf("Starting API server on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Forced shutdown: %v", err)
		os.Exit(1)
	}

	log.Println("Server stopped")
}
