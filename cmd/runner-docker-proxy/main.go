package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/fvrv17/mvp/internal/platform/config"
	"github.com/fvrv17/mvp/internal/runnerproxy"
)

func main() {
	addr := config.String("RUNNER_DOCKER_PROXY_ADDR", ":2375")
	socketPath := config.String("RUNNER_DOCKER_SOCKET", "/var/run/docker.sock")
	sandboxImage := config.String("RUNNER_SANDBOX_IMAGE", "deploy-runner:latest")
	sandboxCommand := config.String("RUNNER_SANDBOX_COMMAND", "node /opt/skillroom-runtime/run-evaluation.mjs")
	maxMemoryMB := config.Int("RUNNER_DOCKER_PROXY_MAX_MEMORY_MB", 512)
	maxPids := config.Int("RUNNER_DOCKER_PROXY_MAX_PIDS", 512)
	readTimeout := config.Duration("RUNNER_DOCKER_PROXY_READ_TIMEOUT", 5*time.Second)
	writeTimeout := config.Duration("RUNNER_DOCKER_PROXY_WRITE_TIMEOUT", 30*time.Second)
	idleTimeout := config.Duration("RUNNER_DOCKER_PROXY_IDLE_TIMEOUT", 60*time.Second)
	shutdownTimeout := config.Duration("RUNNER_DOCKER_PROXY_SHUTDOWN_TIMEOUT", 10*time.Second)

	proxy := runnerproxy.NewWithConfig(runnerproxy.Config{
		SocketPath:      socketPath,
		AllowedImage:    sandboxImage,
		AllowedCommands: []string{sandboxCommand},
		MaxMemoryBytes:  int64(maxMemoryMB) * 1024 * 1024,
		MaxPidsLimit:    int64(maxPids),
	})
	server := &http.Server{
		Addr:         addr,
		Handler:      proxy.Handler(),
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
		log.Printf("runner docker proxy shutdown started")
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("runner docker proxy shutdown error: %v", err)
		}
	}()

	log.Printf("runner docker proxy listening on %s", addr)
	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
