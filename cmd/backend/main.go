package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/fvrv17/mvp/internal/backend"
	runsvc "github.com/fvrv17/mvp/internal/runner"
)

func main() {
	cfg, err := loadStartupConfig()
	if err != nil {
		log.Fatal(err)
	}
	for _, warning := range cfg.InsecureWarnings() {
		log.Printf("WARNING: %s", warning)
	}

	app := backend.NewApp(cfg.Secret, cfg.Issuer)
	if cfg.DatabaseURL != "" {
		persistentApp, err := backend.NewPersistentApp(context.Background(), cfg.Secret, cfg.Issuer, cfg.DatabaseURL)
		if err != nil {
			log.Fatal(err)
		}
		app = persistentApp
	}
	if cfg.RedisAddr != "" {
		app.SetOpsStore(backend.NewRedisOpsStore(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB))
		log.Printf("backend ops store: redis %s db=%d", cfg.RedisAddr, cfg.RedisDB)
	} else {
		log.Printf("backend ops store: in-memory")
	}
	if cfg.OpenAIAPIKey != "" {
		app.SetAIProvider(backend.NewCompositeAIProvider(
			backend.NewOpenAIResponsesProvider(cfg.OpenAIAPIKey, cfg.OpenAIModel, cfg.OpenAIBaseURL, cfg.OpenAIOrganization, cfg.OpenAIProject),
			backend.NewDeterministicAIProvider(),
		))
		log.Printf("backend ai provider: openai model=%s", cfg.OpenAIModel)
	} else {
		log.Printf("backend ai provider: deterministic fallback")
	}
	app.SetChallengeRunner(runsvc.NewHTTPClient(cfg.RunnerBaseURL, cfg.RunnerTimeout))
	log.Printf("backend runner: remote %s", cfg.RunnerBaseURL)

	server := &http.Server{
		Addr:         cfg.Addr,
		Handler:      app.Router(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		log.Printf("backend shutdown started")
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("backend shutdown error: %v", err)
		}
		if err := app.Close(); err != nil {
			log.Printf("backend close error: %v", err)
		}
	}()

	log.Printf("backend listening on %s", cfg.Addr)
	err = server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
