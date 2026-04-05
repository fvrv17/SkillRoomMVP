package main

import (
	"log"
	"net/http"
	"time"

	"github.com/fvrv17/mvp/internal/backend"
	runsvc "github.com/fvrv17/mvp/internal/runner"
)

func main() {
	runnerHandler := runsvc.NewHandler(runsvc.NewDockerEngine(runsvc.DockerConfig{
		SandboxImage: "deploy-runner:latest",
	}))

	app := backend.NewApp("dev-secret", "mvp-platform")
	app.SetChallengeRunner(runsvc.NewHTTPClient("http://127.0.0.1:8081", 3*time.Second))

	errs := make(chan error, 2)
	go serve("runner-service", &http.Server{
		Addr:         ":8081",
		Handler:      runnerHandler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}, errs)
	go serve("backend", &http.Server{
		Addr:         ":8080",
		Handler:      app.Router(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}, errs)

	log.Printf("devstack started: backend=:8080 runner=:8081")
	log.Fatal(<-errs)
}

func serve(name string, server *http.Server, errs chan<- error) {
	log.Printf("%s listening on %s", name, server.Addr)
	errs <- server.ListenAndServe()
}
