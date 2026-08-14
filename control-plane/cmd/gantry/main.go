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

	"github.com/AirSodaz/gantry/internal/adminapi"
	"github.com/AirSodaz/gantry/internal/agentlifecycle"
	"github.com/AirSodaz/gantry/internal/authorization"
	"github.com/AirSodaz/gantry/internal/config"
	"github.com/AirSodaz/gantry/internal/copilotapi"
	"github.com/AirSodaz/gantry/internal/database"
	"github.com/AirSodaz/gantry/internal/development"
	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/objectstore"
	"github.com/AirSodaz/gantry/internal/phase0dev"
	"github.com/AirSodaz/gantry/internal/runnersession"
	"github.com/AirSodaz/gantry/internal/tasks"
	"github.com/jackc/pgx/v5/pgxpool"
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
	databasePool, err := database.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		logger.Error("database is unavailable", "error", err)
		os.Exit(1)
	}
	defer databasePool.Close()
	if err := database.InitializeSchema(context.Background(), databasePool); err != nil {
		logger.Error("database schema initialization failed", "error", err)
		os.Exit(1)
	}
	if cfg.Phase0Dev.Enabled {
		if err := development.Seed(context.Background(), databasePool); err != nil {
			logger.Error("development fixture seed failed", "error", err)
			os.Exit(1)
		}
	}
	taskService := tasks.NewService(databasePool)
	authorizer := authorization.NewService(databasePool)
	agentService := agentlifecycle.NewService(databasePool, authorizer)
	failedRuns, err := taskService.FailInFlight(context.Background(), "control plane restarted while demo run was active")
	if err != nil {
		logger.Error("could not recover interrupted runs", "error", err)
		os.Exit(1)
	}
	if failedRuns != 0 {
		logger.Warn("marked interrupted runs as failed", "count", failedRuns)
	}
	developmentLifecycle := development.NewLifecycle(taskService)
	persistentScheduler := runnersession.NewPersistentScheduler(logger, taskService)
	var copilotAuth *identity.Authenticator
	if cfg.CopilotOIDC.Issuer != "" {
		verifier, err := identity.NewOIDCVerifier(context.Background(), cfg.CopilotOIDC.Issuer, cfg.CopilotOIDC.Audience)
		if err != nil {
			logger.Error("Copilot OIDC configuration is unavailable", "error", err)
			os.Exit(1)
		}
		copilotAuth = identity.NewAuthenticator(verifier, identity.NewResolver(databasePool))
	}
	var adminAuth *identity.Authenticator
	if cfg.AdminOIDC.Issuer != "" {
		verifier, err := identity.NewOIDCVerifier(context.Background(), cfg.AdminOIDC.Issuer, cfg.AdminOIDC.Audience)
		if err != nil {
			logger.Error("Admin OIDC configuration is unavailable", "error", err)
			os.Exit(1)
		}
		adminAuth = identity.NewAuthenticator(verifier, identity.NewResolver(databasePool))
	}

	public := publicServer(cfg, store, databasePool, developmentLifecycle, taskService, agentService, authorizer, persistentScheduler, copilotAuth, adminAuth, logger)
	runner := runnerServer(cfg, logger, persistentScheduler)
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

func publicServer(cfg config.Config, store objectstore.ObjectStore, databasePool *pgxpool.Pool, developmentLifecycle *development.Lifecycle, taskService *tasks.Service, agentService *agentlifecycle.Service, authorizer *authorization.Service, scheduler *runnersession.PersistentScheduler, copilotAuth, adminAuth *identity.Authenticator, logger *slog.Logger) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := database.Ready(r.Context(), databasePool); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
			return
		}
		if err := store.Ready(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	if cfg.Phase0Dev.Enabled {
		mux.Handle("/internal/phase0/", phase0dev.NewHandler(cfg.Phase0Dev.Token, developmentLifecycle, scheduler, logger))
	}
	if copilotAuth != nil {
		mux.Handle("/api/copilot/v1/", http.StripPrefix("/api/copilot/v1", copilotapi.New(copilotAuth, taskService, scheduler, logger)))
	}
	if adminAuth != nil {
		mux.Handle("/api/admin/v1/", http.StripPrefix("/api/admin/v1", adminapi.New(adminAuth, authorizer, agentService, logger)))
	}
	// Product routes are OpenAPI-owned. Connect handlers are registered only below.
	return &http.Server{Addr: cfg.HTTPAddress, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
}

func runnerServer(cfg config.Config, logger *slog.Logger, scheduler runnersession.Coordinator) *http.Server {
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
