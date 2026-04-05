package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/fvrv17/mvp/internal/backend"
	"github.com/fvrv17/mvp/internal/platform/config"
	runsvc "github.com/fvrv17/mvp/internal/runner"
)

func main() {
	addr := config.String("BACKEND_ADDR", ":8080")
	secret := config.String("AUTH_TOKEN_SECRET", "dev-secret")
	issuer := config.String("AUTH_TOKEN_ISSUER", "mvp-platform")
	readTimeout := config.Duration("BACKEND_READ_TIMEOUT", 10*time.Second)
	writeTimeout := config.Duration("BACKEND_WRITE_TIMEOUT", 30*time.Second)
	idleTimeout := config.Duration("BACKEND_IDLE_TIMEOUT", 60*time.Second)
	shutdownTimeout := config.Duration("BACKEND_SHUTDOWN_TIMEOUT", 10*time.Second)
	databaseURL := config.String("DATABASE_URL", "")
	redisAddr := config.String("REDIS_ADDR", "")
	redisPassword := config.String("REDIS_PASSWORD", "")
	redisDB := config.Int("REDIS_DB", 0)
	openAIAPIKey := config.String("OPENAI_API_KEY", "")
	openAIModel := config.String("OPENAI_MODEL", "gpt-4.1-mini")
	openAIBaseURL := config.String("OPENAI_BASE_URL", "https://api.openai.com/v1")
	openAIOrganization := config.String("OPENAI_ORGANIZATION", "")
	openAIProject := config.String("OPENAI_PROJECT", "")
	runnerBaseURL := config.String("RUNNER_BASE_URL", "")
	runnerTimeout := config.Duration("RUNNER_TIMEOUT", 5*time.Second)

	app := backend.NewApp(secret, issuer)
	if databaseURL != "" {
		persistentApp, err := backend.NewPersistentApp(context.Background(), secret, issuer, databaseURL)
		if err != nil {
			log.Fatal(err)
		}
		app = persistentApp
	}
	if redisAddr != "" {
		app.SetOpsStore(backend.NewRedisOpsStore(redisAddr, redisPassword, redisDB))
		log.Printf("backend ops store: redis %s db=%d", redisAddr, redisDB)
	} else {
		log.Printf("backend ops store: in-memory")
	}
	if openAIAPIKey != "" {
		app.SetAIProvider(backend.NewCompositeAIProvider(
			backend.NewOpenAIResponsesProvider(openAIAPIKey, openAIModel, openAIBaseURL, openAIOrganization, openAIProject),
			backend.NewDeterministicAIProvider(),
		))
		log.Printf("backend ai provider: openai model=%s", openAIModel)
	} else {
		log.Printf("backend ai provider: deterministic fallback")
	}
	if runnerBaseURL == "" {
		log.Fatal("RUNNER_BASE_URL is required")
	}
	app.SetChallengeRunner(runsvc.NewHTTPClient(runnerBaseURL, runnerTimeout))
	log.Printf("backend runner: remote %s", runnerBaseURL)

	server := &http.Server{
		Addr:         addr,
		Handler:      app.Router(),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		log.Printf("backend shutdown started")
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("backend shutdown error: %v", err)
		}
		if err := app.Close(); err != nil {
			log.Printf("backend close error: %v", err)
		}
	}()

	log.Printf("backend listening on %s", addr)
	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
