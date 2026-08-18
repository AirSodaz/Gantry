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
	"github.com/AirSodaz/gantry/internal/adminaudit"
	"github.com/AirSodaz/gantry/internal/adminevaluation"
	"github.com/AirSodaz/gantry/internal/adminintegration"
	"github.com/AirSodaz/gantry/internal/adminoverview"
	"github.com/AirSodaz/gantry/internal/adminplatform"
	"github.com/AirSodaz/gantry/internal/adminpolicy"
	"github.com/AirSodaz/gantry/internal/adminruns"
	"github.com/AirSodaz/gantry/internal/agentlifecycle"
	"github.com/AirSodaz/gantry/internal/approvals"
	"github.com/AirSodaz/gantry/internal/authorization"
	"github.com/AirSodaz/gantry/internal/config"
	"github.com/AirSodaz/gantry/internal/configassets"
	"github.com/AirSodaz/gantry/internal/copilotapi"
	"github.com/AirSodaz/gantry/internal/credentials"
	"github.com/AirSodaz/gantry/internal/database"
	"github.com/AirSodaz/gantry/internal/development"
	"github.com/AirSodaz/gantry/internal/developmentapi"
	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/objectstore"
	"github.com/AirSodaz/gantry/internal/runnersession"
	"github.com/AirSodaz/gantry/internal/runs"
	"github.com/AirSodaz/gantry/internal/sessions"
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
	if cfg.Development.Enabled {
		if err := development.Seed(context.Background(), databasePool); err != nil {
			logger.Error("development fixture seed failed", "error", err)
			os.Exit(1)
		}
		if _, err := credentials.NewFileBroker(cfg.DevCredential.File, cfg.DevCredential.Key); err != nil {
			logger.Error("development credential broker configuration is invalid", "error", err)
			os.Exit(1)
		}
	}
	approvalService := approvals.NewService(databasePool)
	runService := runs.NewService(databasePool, approvalService, store)
	sessionService := sessions.NewService(databasePool, approvalService, store, runService)
	authorizer := authorization.NewService(databasePool)
	assetService := configassets.NewService(databasePool, authorizer)
	policyService := adminpolicy.NewService(databasePool, authorizer)
	evaluationService := adminevaluation.NewService(databasePool, authorizer)
	integrationService := adminintegration.NewService(databasePool, authorizer)
	platformService := adminplatform.NewService(databasePool, authorizer)
	agentService := agentlifecycle.NewService(databasePool, authorizer)
	failedRuns, err := runService.FailInFlight(context.Background(), "control plane restarted while a run was active")
	if err != nil {
		logger.Error("could not recover interrupted runs", "error", err)
		os.Exit(1)
	}
	if failedRuns != 0 {
		logger.Warn("marked interrupted runs as failed", "count", failedRuns)
	}
	developmentLifecycle := development.NewLifecycle(sessionService)
	persistentScheduler := runnersession.NewPersistentScheduler(logger, runService)
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

	public := publicServer(cfg, store, databasePool, developmentLifecycle, sessionService, runService, approvalService, agentService, assetService, policyService, evaluationService, integrationService, platformService, authorizer, persistentScheduler, copilotAuth, adminAuth, logger)
	runner := runnerServer(cfg, logger, persistentScheduler)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	approvalExpiry := approvals.NewExpiryWorker(approvalService, persistentScheduler, logger, 5*time.Second)

	errCh := make(chan error, 2)
	go approvalExpiry.Run(ctx)
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

func publicServer(cfg config.Config, store objectstore.ObjectStore, databasePool *pgxpool.Pool, developmentLifecycle *development.Lifecycle, sessionService *sessions.Service, runService *runs.Service, approvalService *approvals.Service, agentService *agentlifecycle.Service, assetService *configassets.Service, policyService *adminpolicy.Service, evaluationService *adminevaluation.Service, integrationService *adminintegration.Service, platformService *adminplatform.Service, authorizer *authorization.Service, scheduler *runnersession.PersistentScheduler, copilotAuth, adminAuth *identity.Authenticator, logger *slog.Logger) *http.Server {
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
	if cfg.Development.Enabled {
		mux.Handle("/internal/development/", developmentapi.NewHandler(cfg.Development.Token, developmentLifecycle, scheduler, logger))
	}
	mux.HandleFunc("POST /internal/runner/artifacts/{artifactID}", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 64<<20+1)
		if err := runService.UploadArtifact(r.Context(), r.PathValue("artifactID"), r.Header.Get("X-Gantry-Artifact-Token"), r.Body); err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "artifact upload rejected"})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"status": "available", "artifact_id": r.PathValue("artifactID")})
	})
	if copilotAuth != nil {
		mux.Handle("/api/copilot/v1/", http.StripPrefix("/api/copilot/v1", copilotapi.New(copilotAuth, sessionService, approvalService, scheduler, logger, runService)))
	}
	if adminAuth != nil {
		overviewService := adminoverview.NewService(databasePool, authorizer)
		runService := adminruns.NewService(databasePool, authorizer)
		artifactStore, _ := store.(objectstore.ArtifactStore)
		auditService := adminaudit.NewServiceWithStore(databasePool, authorizer, artifactStore)
		mux.Handle("/api/admin/v1/", http.StripPrefix("/api/admin/v1", adminapi.NewWithTargetAuditPolicyEvaluationPlatform(adminAuth, authorizer, agentService, agentService, assetService, overviewService, runService, auditService, policyService, evaluationService, integrationService, platformService, logger)))
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
