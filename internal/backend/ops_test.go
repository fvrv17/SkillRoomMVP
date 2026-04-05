package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMemoryOpsStoreAllowAndCache(t *testing.T) {
	store := NewMemoryOpsStore()
	ctx := context.Background()

	first, err := store.Allow(ctx, "rate:test", 2, time.Minute)
	if err != nil {
		t.Fatalf("allow first: %v", err)
	}
	if !first.Allowed || first.Remaining != 1 {
		t.Fatalf("unexpected first decision: %+v", first)
	}

	second, err := store.Allow(ctx, "rate:test", 2, time.Minute)
	if err != nil {
		t.Fatalf("allow second: %v", err)
	}
	if !second.Allowed || second.Remaining != 0 {
		t.Fatalf("unexpected second decision: %+v", second)
	}

	third, err := store.Allow(ctx, "rate:test", 2, time.Minute)
	if err != nil {
		t.Fatalf("allow third: %v", err)
	}
	if third.Allowed {
		t.Fatalf("expected third decision to be blocked")
	}

	payload := map[string]any{"score": 88.5}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if err := store.Set(ctx, "cache:test", encoded, time.Minute); err != nil {
		t.Fatalf("set cache: %v", err)
	}
	cached, ok, err := store.Get(ctx, "cache:test")
	if err != nil {
		t.Fatalf("get cache: %v", err)
	}
	if !ok {
		t.Fatalf("expected cache hit")
	}
	var decoded map[string]any
	if err := json.Unmarshal(cached, &decoded); err != nil {
		t.Fatalf("decode cache: %v", err)
	}
	if decoded["score"].(float64) != 88.5 {
		t.Fatalf("unexpected cached payload: %+v", decoded)
	}
}

func TestRegisterRateLimit(t *testing.T) {
	app := NewApp("test-secret", "test-issuer")
	router := app.Router()

	var lastCode int
	for i := 0; i < 11; i++ {
		body := RegisterRequest{
			Email:    "same@example.com",
			Username: "same",
			Password: "password123",
			Country:  "US",
		}
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", encodeJSONBody(t, body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "203.0.113.10:12345"
		router.ServeHTTP(recorder, req)
		lastCode = recorder.Code
	}

	if lastCode != http.StatusTooManyRequests {
		t.Fatalf("expected rate-limited request, got %d", lastCode)
	}
}

func encodeJSONBody(t *testing.T, value any) *bytes.Buffer {
	t.Helper()
	var buffer bytes.Buffer
	if err := json.NewEncoder(&buffer).Encode(value); err != nil {
		t.Fatalf("encode json body: %v", err)
	}
	return &buffer
}
