package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oai404iao/sub_manager/internal/config"
	"github.com/oai404iao/sub_manager/internal/httpapi"
	"github.com/oai404iao/sub_manager/internal/store"
	"github.com/oai404iao/sub_manager/internal/version"
)

func main() {
	showVersion := flag.Bool("version", false, "print version information and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version.String())
		return
	}

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
		log.Printf("Sub Manager %s listening on %s", version.Current().Version, cfg.Addr)
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
