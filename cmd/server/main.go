package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"submanager/internal/config"
	"submanager/internal/httpapi"
	"submanager/internal/store"
)

func main() {
	cfg := config.Load()
	database, err := store.Open(cfg.DBPath, cfg.AdminUser, cfg.AdminPassword)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.New(cfg, database).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("Sub Manager listening on %s", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	timeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(timeout); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
		_ = server.Close()
	}
}
