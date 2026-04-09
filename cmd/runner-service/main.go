package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/fvrv17/mvp/internal/platform/config"
	runsvc "github.com/fvrv17/mvp/internal/runner"
)

func main() {
	addr := config.String("RUNNER_ADDR", ":8081")
	readTimeout := config.Duration("RUNNER_READ_TIMEOUT", 10*time.Second)
	writeTimeout := config.Duration("RUNNER_WRITE_TIMEOUT", 30*time.Second)
	idleTimeout := config.Duration("RUNNER_IDLE_TIMEOUT", 60*time.Second)
	shutdownTimeout := config.Duration("RUNNER_SHUTDOWN_TIMEOUT", 10*time.Second)
	sandboxImage := config.String("RUNNER_SANDBOX_IMAGE", "deploy-runner:latest")
	sandboxCommand := config.String("RUNNER_SANDBOX_COMMAND", "node /opt/skillroom-runtime/run-evaluation.mjs")
	sandboxUser := config.String("RUNNER_SANDBOX_USER", "1000:1000")
	dockerHost := config.String("RUNNER_DOCKER_HOST", "")
	cpuLimit := config.String("RUNNER_CPU_LIMIT", "0.50")
	memoryMB := config.Int("RUNNER_MEMORY_MB", 256)
	defaultTimeout := config.Duration("RUNNER_DEFAULT_TIMEOUT", 6*time.Second)

	engine := runsvc.NewDockerEngine(runsvc.DockerConfig{
		DockerHost:     dockerHost,
		SandboxImage:   sandboxImage,
		SandboxCommand: sandboxCommand,
		SandboxUser:    sandboxUser,
		DefaultCPU:     cpuLimit,
		DefaultMemory:  memoryMB,
		DefaultTimeout: defaultTimeout,
	})

	server := &http.Server{
		Addr:         addr,
		Handler:      runsvc.NewHandler(engine),
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
		log.Printf("runner shutdown started")
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("runner shutdown error: %v", err)
		}
	}()

	log.Printf("runner listening on %s", addr)
	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
