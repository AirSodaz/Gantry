package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AirSodaz/gantry/internal/config"
	"github.com/AirSodaz/gantry/internal/objectstore"
	"github.com/AirSodaz/gantry/internal/runnersession"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	store, err := objectstore.NewS3(cfg.ObjectStorage)
	if err != nil {
		logger.Error("invalid object storage configuration", "error", err)
		os.Exit(1)
	}

	public := publicServer(cfg, store)
	runner := runnerServer(cfg, logger, runnersession.NewScheduler())
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 2)
	go serve(errCh, "public HTTP", public)
	go serve(errCh, "runner gRPC", runner)

	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
	case err := <-errCh:
		logger.Error("server stopped unexpectedly", "error", err)
		stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = public.Shutdown(shutdownCtx)
	_ = runner.Shutdown(shutdownCtx)
}

func serve(errCh chan<- error, name string, server *http.Server) {
	var err error
	if server.TLSConfig != nil {
		err = server.ListenAndServeTLS("", "")
	} else {
		err = server.ListenAndServe()
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		errCh <- errors.New(name + ": " + err.Error())
	}
}

func publicServer(cfg config.Config, store objectstore.ObjectStore) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := store.Ready(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	// Public routes are OpenAPI-owned. Connect handlers are registered only below.
	return &http.Server{Addr: cfg.HTTPAddress, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
}

func runnerServer(cfg config.Config, logger *slog.Logger, scheduler *runnersession.Scheduler) *http.Server {
	mux := http.NewServeMux()
	path, handler := runnersession.NewHandler(logger, scheduler)
	mux.Handle(path, handler)
	server := &http.Server{Addr: cfg.GRPCAddress, Handler: h2c.NewHandler(mux, &http2.Server{}), ReadHeaderTimeout: 5 * time.Second}
	if cfg.RunnerTLS.CertificateFile == "" {
		return server
	}
	certificate, err := tls.LoadX509KeyPair(cfg.RunnerTLS.CertificateFile, cfg.RunnerTLS.KeyFile)
	if err != nil {
		panic(err)
	}
	ca, err := os.ReadFile(cfg.RunnerTLS.ClientCAFile)
	if err != nil {
		panic(err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		panic("runner client CA is invalid")
	}
	server.Handler = mux
	server.TLSConfig = &tls.Config{Certificates: []tls.Certificate{certificate}, ClientCAs: pool, ClientAuth: tls.RequireAndVerifyClientCert, NextProtos: []string{"h2"}, MinVersion: tls.VersionTLS13}
	return server
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
