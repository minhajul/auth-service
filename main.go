package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	serviceName = "auth-service"
	apiVersion  = "v1.0.0"
)

// healthResponse is the JSON payload returned by /health.
type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// infoResponse is the JSON payload returned by /api/v1/info.
type infoResponse struct {
	ServiceName string `json:"service_name"`
	Message     string `json:"message"`
	APIVersion  string `json:"api_version"`
}

// writeJSON serialises v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}

func main() {
	mux := http.NewServeMux()

	// GET /health — liveness/readiness probe.
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{
			Status:  "healthy",
			Service: serviceName,
		})
	})

	// GET /api/v1/info — service self-description.
	mux.HandleFunc("GET /api/v1/info", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, infoResponse{
			ServiceName: serviceName,
			Message:     "Welcome to the " + serviceName + " API",
			APIVersion:  apiVersion,
		})
	})

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Run the server in a goroutine so the main goroutine can wait on signals.
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("%s listening on %s", serviceName, srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	// Wait for an interrupt signal or a fatal server error.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		if err != nil {
			log.Fatalf("server failed: %v", err)
		}
	case sig := <-quit:
		log.Printf("received signal %s, shutting down gracefully...", sig)
	}

	// Give in-flight requests up to 30 seconds to finish before forcing exit.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
		if err := srv.Close(); err != nil {
			log.Printf("force close failed: %v", err)
		}
		os.Exit(1)
	}

	log.Printf("%s stopped", serviceName)
}
