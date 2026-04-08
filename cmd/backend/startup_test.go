package main

import "testing"

func TestLoadStartupConfigRequiresProductionDependenciesByDefault(t *testing.T) {
	t.Setenv("RUNNER_BASE_URL", "http://runner:8081")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("AUTH_TOKEN_SECRET", "dev-secret")
	t.Setenv("ALLOW_INSECURE_BOOT", "")

	_, err := loadStartupConfig()
	if err == nil {
		t.Fatalf("expected secure startup validation to fail")
	}
}

func TestLoadStartupConfigAllowsExplicitInsecureBoot(t *testing.T) {
	t.Setenv("RUNNER_BASE_URL", "http://runner:8081")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("AUTH_TOKEN_SECRET", "dev-secret")
	t.Setenv("ALLOW_INSECURE_BOOT", "true")

	cfg, err := loadStartupConfig()
	if err != nil {
		t.Fatalf("expected insecure boot override to pass: %v", err)
	}
	if !cfg.AllowInsecureBoot {
		t.Fatalf("expected insecure boot flag to be enabled")
	}
}

func TestLoadStartupConfigRequiresRunnerAlways(t *testing.T) {
	t.Setenv("RUNNER_BASE_URL", "")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/mvp?sslmode=disable")
	t.Setenv("REDIS_ADDR", "localhost:6379")
	t.Setenv("AUTH_TOKEN_SECRET", "skillroom-local-secret")
	t.Setenv("ALLOW_INSECURE_BOOT", "true")

	_, err := loadStartupConfig()
	if err == nil {
		t.Fatalf("expected runner requirement to fail startup validation")
	}
}
