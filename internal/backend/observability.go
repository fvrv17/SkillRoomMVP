package backend

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fvrv17/mvp/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type AppMetrics struct {
	activeRequests int64
	requestsTotal  uint64

	mu          sync.RWMutex
	stats       map[metricKey]*metricValue
	eventTotals map[string]uint64
}

type metricKey struct {
	Method string
	Route  string
	Status int
}

type metricValue struct {
	Count      uint64
	DurationNS uint64
	Bytes      uint64
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func NewAppMetrics() *AppMetrics {
	return &AppMetrics{
		stats:       map[metricKey]*metricValue{},
		eventTotals: map[string]uint64{},
	}
}

func (a *AppMetrics) Observe(method, route string, status, bytes int, duration time.Duration) {
	atomic.AddUint64(&a.requestsTotal, 1)
	key := metricKey{
		Method: method,
		Route:  route,
		Status: status,
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	entry, ok := a.stats[key]
	if !ok {
		entry = &metricValue{}
		a.stats[key] = entry
	}
	entry.Count++
	entry.DurationNS += uint64(duration.Nanoseconds())
	entry.Bytes += uint64(bytes)
}

func (a *AppMetrics) IncrementEvent(name string) {
	if strings.TrimSpace(name) == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.eventTotals[name]++
}

func (a *AppMetrics) Text() string {
	var builder strings.Builder
	builder.WriteString("# HELP backend_active_requests Active in-flight HTTP requests.\n")
	builder.WriteString("# TYPE backend_active_requests gauge\n")
	builder.WriteString("backend_active_requests ")
	builder.WriteString(strconv.FormatInt(atomic.LoadInt64(&a.activeRequests), 10))
	builder.WriteByte('\n')

	builder.WriteString("# HELP backend_requests_total Total HTTP requests handled.\n")
	builder.WriteString("# TYPE backend_requests_total counter\n")
	builder.WriteString("backend_requests_total ")
	builder.WriteString(strconv.FormatUint(atomic.LoadUint64(&a.requestsTotal), 10))
	builder.WriteByte('\n')

	builder.WriteString("# HELP backend_http_requests_total HTTP requests partitioned by method, route, and status.\n")
	builder.WriteString("# TYPE backend_http_requests_total counter\n")
	builder.WriteString("# HELP backend_http_request_duration_seconds_sum Total request duration in seconds.\n")
	builder.WriteString("# TYPE backend_http_request_duration_seconds_sum counter\n")
	builder.WriteString("# HELP backend_http_response_size_bytes_total Total response bytes written.\n")
	builder.WriteString("# TYPE backend_http_response_size_bytes_total counter\n")
	builder.WriteString("# HELP backend_domain_events_total Domain and runtime event counters.\n")
	builder.WriteString("# TYPE backend_domain_events_total counter\n")

	a.mu.RLock()
	keys := make([]metricKey, 0, len(a.stats))
	for key := range a.stats {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Route == keys[j].Route {
			if keys[i].Method == keys[j].Method {
				return keys[i].Status < keys[j].Status
			}
			return keys[i].Method < keys[j].Method
		}
		return keys[i].Route < keys[j].Route
	})
	for _, key := range keys {
		value := a.stats[key]
		labels := fmt.Sprintf(`method=%q,route=%q,status=%q`, key.Method, key.Route, strconv.Itoa(key.Status))
		builder.WriteString("backend_http_requests_total{")
		builder.WriteString(labels)
		builder.WriteString("} ")
		builder.WriteString(strconv.FormatUint(value.Count, 10))
		builder.WriteByte('\n')

		builder.WriteString("backend_http_request_duration_seconds_sum{")
		builder.WriteString(labels)
		builder.WriteString("} ")
		builder.WriteString(strconv.FormatFloat(float64(value.DurationNS)/float64(time.Second), 'f', 6, 64))
		builder.WriteByte('\n')

		builder.WriteString("backend_http_response_size_bytes_total{")
		builder.WriteString(labels)
		builder.WriteString("} ")
		builder.WriteString(strconv.FormatUint(value.Bytes, 10))
		builder.WriteByte('\n')
	}

	eventKeys := make([]string, 0, len(a.eventTotals))
	for key := range a.eventTotals {
		eventKeys = append(eventKeys, key)
	}
	sort.Strings(eventKeys)
	for _, key := range eventKeys {
		builder.WriteString("backend_domain_events_total{event=")
		builder.WriteString(strconv.Quote(key))
		builder.WriteString("} ")
		builder.WriteString(strconv.FormatUint(a.eventTotals[key], 10))
		builder.WriteByte('\n')
	}
	a.mu.RUnlock()
	return builder.String()
}

func (a *App) observeRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		atomic.AddInt64(&a.metrics.activeRequests, 1)
		defer atomic.AddInt64(&a.metrics.activeRequests, -1)

		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)

		route := r.URL.Path
		if routeContext := chi.RouteContext(r.Context()); routeContext != nil {
			if pattern := routeContext.RoutePattern(); pattern != "" {
				route = pattern
			}
		}
		duration := time.Since(startedAt)
		a.metrics.Observe(r.Method, route, recorder.status, recorder.bytes, duration)

		requestID := middleware.GetReqID(r.Context())
		remoteIP := a.realIP(r)
		log.Printf(
			"request_id=%s method=%s route=%s status=%d duration_ms=%d bytes=%d remote_ip=%s",
			requestID,
			r.Method,
			route,
			recorder.status,
			duration.Milliseconds(),
			recorder.bytes,
			remoteIP,
		)
	})
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(payload []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(payload)
	r.bytes += n
	return n, err
}

func (a *App) handleLiveness(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) handleReadiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := a.ready(ctx); err != nil {
		a.metrics.IncrementEvent("readiness_failed")
		httpx.WriteError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (a *App) ready(ctx context.Context) error {
	if a.runner == nil {
		return fmt.Errorf("runner unavailable: engine is not configured")
	}
	type readinessChecker interface {
		Ready(context.Context) error
	}
	if checker, ok := a.runner.(readinessChecker); ok {
		if err := checker.Ready(ctx); err != nil {
			return fmt.Errorf("runner unavailable: %w", err)
		}
	}
	if a.store != nil {
		if err := a.store.Ping(ctx); err != nil {
			return fmt.Errorf("postgres unavailable: %w", err)
		}
	}
	if a.ops != nil {
		if err := a.ops.Ping(ctx); err != nil {
			return fmt.Errorf("ops store unavailable: %w", err)
		}
	}
	return nil
}

func (a *App) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(a.metrics.Text()))
}

func (a *App) realIP(r *http.Request) string {
	remoteIP := socketIP(r)
	if !a.trustForwardedHeaders(r, remoteIP) {
		return remoteIP
	}
	for _, header := range []string{"X-Forwarded-For", "X-Real-IP"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value == "" {
			continue
		}
		if header == "X-Forwarded-For" {
			return strings.TrimSpace(strings.Split(value, ",")[0])
		}
		return value
	}
	return remoteIP
}

func (a *App) trustForwardedHeaders(r *http.Request, remoteIP string) bool {
	if r == nil {
		return false
	}
	if a.trustedProxySecret != "" {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get(proxySecretHeaderName)), []byte(a.trustedProxySecret)) == 1 {
			return true
		}
	}
	if len(a.trustedProxyCIDRs) == 0 {
		return false
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(remoteIP))
	if err != nil {
		return false
	}
	for _, prefix := range a.trustedProxyCIDRs {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func socketIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	hostPort := strings.TrimSpace(r.RemoteAddr)
	if hostPort == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(hostPort)
	if err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(hostPort, "[]")
}
