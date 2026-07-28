package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"keeper/docs"
	"keeper/internal/app"
	"keeper/internal/audit"
	"keeper/internal/db"
	"keeper/internal/division"
	"keeper/internal/guestkey"
	"keeper/internal/impersonation"
	platformhttp "keeper/internal/platform/http"
	"keeper/internal/policy"
	"keeper/internal/user"
	"keeper/pkg/auth"
	"keeper/pkg/config"
	"keeper/pkg/httpclient"

	"github.com/go-chi/chi/v5"
)

// @title Keeper API
// @version 1.0
// @description This is a microservice for user management.
// @host localhost:8080
// @BasePath /

// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

// impersonationCodeTTL bounds how long a one-time handoff code is valid before
// it must be exchanged for a token. Kept very short — the code only needs to
// survive the redirect into the target service UI.
const impersonationCodeTTL = 60 * time.Second

func main() {
	checkConfig := flag.Bool("check-config", false, "validate configuration (including secondary listeners) and exit")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("failed to load config: %v\n", err)
		os.Exit(1)
	}

	if *checkConfig {
		enabled := 0
		for i := range cfg.Secondary {
			sec := &cfg.Secondary[i]
			if !sec.Enabled {
				continue
			}
			enabled++
			if err := platformhttp.ValidateRoutes(sec.Routes); err != nil {
				fmt.Printf("config invalid: %s: %v\n", sec.Name, err)
				os.Exit(1)
			}
		}
		fmt.Printf("config OK: primary %s, %d secondary listener(s) enabled, %d impersonation service(s) registered\n", cfg.Server.Addr, enabled, len(cfg.Services))
		os.Exit(0)
	}

	if err := os.MkdirAll(cfg.Log.Dir, 0755); err != nil {
		fmt.Printf("failed to create log directory: %v\n", err)
		os.Exit(1)
	}

	logFile, err := os.OpenFile(filepath.Join(cfg.Log.Dir, "api.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("failed to open log file: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := logFile.Close(); err != nil {
			fmt.Printf("failed to close log file: %v\n", err)
		}
	}()

	var logLevel slog.Level
	switch cfg.Log.Level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	mw := io.MultiWriter(os.Stdout, logFile)
	logger := slog.New(slog.NewJSONHandler(mw, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	// Audit trail is a separate file from api.log — who-changed-what should
	// stay readable on its own, not interleaved with request/debug noise.
	auditLogFile, err := os.OpenFile(filepath.Join(cfg.Log.Dir, "audit.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("failed to open audit log file: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := auditLogFile.Close(); err != nil {
			slog.Error("failed to close audit log file", "error", err)
		}
	}()
	auditLogger := slog.New(slog.NewJSONHandler(auditLogFile, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Override Swagger host
	docs.SwaggerInfo.Host = cfg.Server.Host

	client, dbDriver, err := db.NewClient(cfg.DB)
	if err != nil {
		slog.Error("failed to open database client", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := client.Close(); err != nil {
			slog.Error("failed to close database client", "error", err)
		}
	}()
	client.Use(audit.Hook(auditLogger))

	if err := db.Seed(context.Background(), client, cfg.Seed); err != nil {
		slog.Error("failed to seed initial data", "error", err)
		os.Exit(1)
	}

	// Auth setup
	jwtManager := auth.NewJWTManager(cfg.Auth.JWTSecret, cfg.Auth.JWTExpiry)

	// Login resolves each user's roles from falcon before minting the JWT
	// (fail-closed if falcon is unreachable) — see internal/user.Service.Authenticate.
	falconHTTPClient := httpclient.New(httpclient.Config{Timeout: cfg.Falcon.Timeout, Name: "falcon-s2s"})
	roleResolver := user.NewFalconRoleResolver(falconHTTPClient, cfg.Falcon.BaseURL, jwtManager)

	// Tier 1 (coarse CRUD) authorization cache: role->permission map pulled
	// from falcon and refreshed on a TTL. Warm it eagerly so the first request
	// after boot isn't served against an empty (fail-closed) map; a warm
	// failure only means falcon isn't reachable yet, not that keeper can't
	// boot, so it's logged rather than fatal.
	policyFetcher := policy.NewFetcher(falconHTTPClient, cfg.Falcon.BaseURL, cfg.Falcon.ServiceID, jwtManager)
	policyStore := policy.NewStore(policyFetcher, cfg.Cache.PolicyTTL)
	if err := policyStore.Warm(context.Background()); err != nil {
		slog.Warn("policy cache: startup warm failed, serving fail-closed until falcon is reachable", "error", err)
	}

	// Initialize components
	userRepo := user.NewUserRepository(client)
	userSvc := user.NewUserService(userRepo, jwtManager, roleResolver)
	userHandler := user.NewUserHandler(userSvc)

	divisionRepo := division.NewDivisionRepository(client)
	divisionSvc := division.NewDivisionService(divisionRepo)
	divisionHandler := division.NewDivisionHandler(divisionSvc)

	// Guest tokens are signed with a dedicated secret so they only work on
	// surfaces that explicitly verify with it (e.g. ant's order-intake).
	guestJWTManager := auth.NewJWTManager(cfg.Auth.GuestJWTSecret, cfg.Auth.GuestJWTExpiry)
	guestKeyRepo := guestkey.NewGuestKeyRepository(client)
	guestKeySvc := guestkey.NewGuestKeyService(guestKeyRepo, guestJWTManager, cfg.Auth.GuestJWTExpiry)
	guestKeyHandler := guestkey.NewGuestKeyHandler(guestKeySvc)

	// appSvc depends on guestKeySvc to resolve publishable site keys to app IDs
	// for the public /apps/lookup profile endpoint.
	appRepo := app.NewAppRepository(client)
	appSvc := app.NewAppService(appRepo, guestKeySvc)
	appHandler := app.NewAppHandler(appSvc)

	// Impersonation tokens are signed with their own dedicated secret so they
	// only verify on services explicitly configured with it (and never on
	// primary/guest surfaces). The audience set is the registered service list.
	impJWTManager := auth.NewJWTManager(cfg.Auth.ImpersonationJWTSecret, cfg.Auth.ImpersonationJWTExpiry)
	impServices := make([]impersonation.ServiceInfo, len(cfg.Services))
	for i := range cfg.Services {
		impServices[i] = impersonation.ServiceInfo{
			Key:           cfg.Services[i].Key,
			Audience:      cfg.Services[i].Audience,
			UIExchangeURL: cfg.Services[i].UIExchangeURL,
		}
	}
	impersonationRepo := impersonation.NewImpersonationRepository(client)
	impersonationSvc := impersonation.NewImpersonationService(impersonationRepo, impJWTManager, cfg.Auth.ImpersonationJWTExpiry, impersonationCodeTTL, impServices)
	impersonationHandler := impersonation.NewImpersonationHandler(impersonationSvc)

	router := platformhttp.NewRouter(userHandler, appHandler, divisionHandler, guestKeyHandler, impersonationHandler, jwtManager, cfg, dbDriver)

	srv := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		slog.Info("starting server", "addr", srv.Addr, "env", cfg.Environment)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("failed to listen and serve", "error", err)
			os.Exit(1)
		}
	}()

	// Secondary listeners reuse the same handlers via the mount hook; the
	// entity routers carry their own JWT protection, verified with the
	// listener's manager (primary secret, or per-listener JWT_SECRET).
	mount := func(r chi.Router, jm *auth.JWTManager) {
		r.Mount("/users", userHandler.Routes(jm))
		r.Mount("/managers", userHandler.ManagerRoutes(jm))
		r.Mount("/apps", appHandler.Routes(jm))
		r.Mount("/divisions", divisionHandler.Routes(jm))
		r.Mount("/guest-keys", guestKeyHandler.Routes(jm))
		r.Mount("/impersonations", impersonationHandler.Routes(jm))
	}

	var secondarySrvs []*http.Server
	for i := range cfg.Secondary {
		sec := &cfg.Secondary[i]
		if !sec.Enabled {
			continue
		}

		secondaryRouter, err := platformhttp.NewSecondaryRouter(cfg, sec, jwtManager, mount, dbDriver)
		if err != nil {
			slog.Error("failed to build secondary router", "name", sec.Name, "error", err)
			os.Exit(1)
		}

		secondarySrv := &http.Server{
			Addr:         sec.Addr,
			Handler:      secondaryRouter,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
			IdleTimeout:  cfg.Server.IdleTimeout,
		}
		secondarySrvs = append(secondarySrvs, secondarySrv)

		go func() {
			slog.Info("starting secondary server", "name", sec.Name, "addr", secondarySrv.Addr, "routes", sec.Routes)
			if err := secondarySrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("failed to listen and serve on secondary", "name", sec.Name, "error", err)
				os.Exit(1)
			}
		}()
	}

	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 10 seconds.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	for _, secondarySrv := range secondarySrvs {
		if err := secondarySrv.Shutdown(ctx); err != nil {
			slog.Error("secondary server forced to shutdown", "addr", secondarySrv.Addr, "error", err)
			os.Exit(1)
		}
	}

	slog.Info("server exited gracefully")
}
