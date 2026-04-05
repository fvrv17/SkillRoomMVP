package backend

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RateLimitDecision struct {
	Allowed   bool
	Limit     int
	Remaining int
	ResetAt   time.Time
}

type OpsStore interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (RateLimitDecision, error)
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
	Ping(ctx context.Context) error
}

type MemoryOpsStore struct {
	mu       sync.Mutex
	counters map[string]memoryCounter
	cache    map[string]memoryCacheEntry
}

type memoryCounter struct {
	Count     int
	ExpiresAt time.Time
}

type memoryCacheEntry struct {
	Value     []byte
	ExpiresAt time.Time
}

func NewMemoryOpsStore() *MemoryOpsStore {
	return &MemoryOpsStore{
		counters: map[string]memoryCounter{},
		cache:    map[string]memoryCacheEntry{},
	}
}

func (m *MemoryOpsStore) Allow(_ context.Context, key string, limit int, window time.Duration) (RateLimitDecision, error) {
	now := time.Now().UTC()
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanupExpiredLocked(now)
	entry := m.counters[key]
	if entry.ExpiresAt.IsZero() || !entry.ExpiresAt.After(now) {
		entry = memoryCounter{Count: 0, ExpiresAt: now.Add(window)}
	}
	entry.Count++
	m.counters[key] = entry

	remaining := limit - entry.Count
	if remaining < 0 {
		remaining = 0
	}
	return RateLimitDecision{
		Allowed:   entry.Count <= limit,
		Limit:     limit,
		Remaining: remaining,
		ResetAt:   entry.ExpiresAt,
	}, nil
}

func (m *MemoryOpsStore) Get(_ context.Context, key string) ([]byte, bool, error) {
	now := time.Now().UTC()
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanupExpiredLocked(now)
	entry, ok := m.cache[key]
	if !ok {
		return nil, false, nil
	}
	return append([]byte(nil), entry.Value...), true, nil
}

func (m *MemoryOpsStore) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	now := time.Now().UTC()
	m.mu.Lock()
	defer m.mu.Unlock()

	expiresAt := now.Add(ttl)
	if ttl <= 0 {
		expiresAt = now.Add(30 * time.Second)
	}
	m.cache[key] = memoryCacheEntry{
		Value:     append([]byte(nil), value...),
		ExpiresAt: expiresAt,
	}
	return nil
}

func (m *MemoryOpsStore) Delete(_ context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, key := range keys {
		delete(m.cache, key)
		delete(m.counters, key)
	}
	return nil
}

func (m *MemoryOpsStore) Ping(_ context.Context) error {
	return nil
}

func (m *MemoryOpsStore) cleanupExpiredLocked(now time.Time) {
	for key, entry := range m.counters {
		if !entry.ExpiresAt.After(now) {
			delete(m.counters, key)
		}
	}
	for key, entry := range m.cache {
		if !entry.ExpiresAt.After(now) {
			delete(m.cache, key)
		}
	}
}

type RedisOpsStore struct {
	addr     string
	password string
	db       int
	timeout  time.Duration
}

func NewRedisOpsStore(addr, password string, db int) *RedisOpsStore {
	return &RedisOpsStore{
		addr:     strings.TrimSpace(addr),
		password: password,
		db:       db,
		timeout:  1200 * time.Millisecond,
	}
}

func (r *RedisOpsStore) Allow(ctx context.Context, key string, limit int, window time.Duration) (RateLimitDecision, error) {
	incrResp, err := r.do(ctx, []byte("INCR"), []byte(key))
	if err != nil {
		return RateLimitDecision{}, err
	}
	count := int(incrResp.Integer)
	if count == 1 {
		if _, err := r.do(ctx, []byte("EXPIRE"), []byte(key), []byte(strconv.Itoa(int(window.Seconds())))); err != nil {
			return RateLimitDecision{}, err
		}
	}
	ttlResp, err := r.do(ctx, []byte("TTL"), []byte(key))
	if err != nil {
		return RateLimitDecision{}, err
	}
	ttlSeconds := ttlResp.Integer
	if ttlSeconds < 0 {
		ttlSeconds = int(window.Seconds())
	}
	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}
	return RateLimitDecision{
		Allowed:   count <= limit,
		Limit:     limit,
		Remaining: remaining,
		ResetAt:   time.Now().UTC().Add(time.Duration(ttlSeconds) * time.Second),
	}, nil
}

func (r *RedisOpsStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	resp, err := r.do(ctx, []byte("GET"), []byte(key))
	if err != nil {
		return nil, false, err
	}
	if resp.IsNil {
		return nil, false, nil
	}
	return resp.Bulk, true, nil
}

func (r *RedisOpsStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	seconds := int(ttl.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	_, err := r.do(ctx, []byte("SET"), []byte(key), value, []byte("EX"), []byte(strconv.Itoa(seconds)))
	return err
}

func (r *RedisOpsStore) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	args := make([][]byte, 0, len(keys)+1)
	args = append(args, []byte("DEL"))
	for _, key := range keys {
		args = append(args, []byte(key))
	}
	_, err := r.do(ctx, args...)
	return err
}

func (r *RedisOpsStore) Ping(ctx context.Context) error {
	_, err := r.do(ctx, []byte("PING"))
	return err
}

type redisResponse struct {
	Simple  string
	Bulk    []byte
	Integer int
	IsNil   bool
}

func (r *RedisOpsStore) do(ctx context.Context, args ...[]byte) (redisResponse, error) {
	if strings.TrimSpace(r.addr) == "" {
		return redisResponse{}, errors.New("redis address is required")
	}
	timeout := r.timeout
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	conn, err := net.DialTimeout("tcp", r.addr, timeout)
	if err != nil {
		return redisResponse{}, err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return redisResponse{}, err
	}
	reader := bufio.NewReader(conn)

	if r.password != "" {
		if _, err := writeRedisCommand(conn, []byte("AUTH"), []byte(r.password)); err != nil {
			return redisResponse{}, err
		}
		if _, err := readRedisResponse(reader); err != nil {
			return redisResponse{}, err
		}
	}
	if r.db > 0 {
		if _, err := writeRedisCommand(conn, []byte("SELECT"), []byte(strconv.Itoa(r.db))); err != nil {
			return redisResponse{}, err
		}
		if _, err := readRedisResponse(reader); err != nil {
			return redisResponse{}, err
		}
	}

	if _, err := writeRedisCommand(conn, args...); err != nil {
		return redisResponse{}, err
	}
	return readRedisResponse(reader)
}

func writeRedisCommand(w io.Writer, args ...[]byte) (int, error) {
	var buffer bytes.Buffer
	buffer.WriteString("*")
	buffer.WriteString(strconv.Itoa(len(args)))
	buffer.WriteString("\r\n")
	for _, arg := range args {
		buffer.WriteString("$")
		buffer.WriteString(strconv.Itoa(len(arg)))
		buffer.WriteString("\r\n")
		buffer.Write(arg)
		buffer.WriteString("\r\n")
	}
	return w.Write(buffer.Bytes())
}

func readRedisResponse(reader *bufio.Reader) (redisResponse, error) {
	prefix, err := reader.ReadByte()
	if err != nil {
		return redisResponse{}, err
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return redisResponse{}, err
	}
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")

	switch prefix {
	case '+':
		return redisResponse{Simple: line}, nil
	case '-':
		return redisResponse{}, errors.New(line)
	case ':':
		parsed, err := strconv.Atoi(line)
		if err != nil {
			return redisResponse{}, err
		}
		return redisResponse{Integer: parsed}, nil
	case '$':
		length, err := strconv.Atoi(line)
		if err != nil {
			return redisResponse{}, err
		}
		if length < 0 {
			return redisResponse{IsNil: true}, nil
		}
		payload := make([]byte, length+2)
		if _, err := io.ReadFull(reader, payload); err != nil {
			return redisResponse{}, err
		}
		return redisResponse{Bulk: payload[:length]}, nil
	default:
		return redisResponse{}, fmt.Errorf("unsupported redis response prefix %q", prefix)
	}
}

func cacheJSON[T any](ctx context.Context, store OpsStore, key string, ttl time.Duration, compute func() (T, error)) (T, error) {
	var zero T
	if store != nil {
		payload, ok, err := store.Get(ctx, key)
		if err != nil {
			return zero, err
		}
		if ok {
			var cached T
			if err := json.Unmarshal(payload, &cached); err == nil {
				return cached, nil
			}
		}
	}

	value, err := compute()
	if err != nil {
		return zero, err
	}
	if store != nil {
		if payload, err := json.Marshal(value); err == nil {
			_ = store.Set(ctx, key, payload, ttl)
		}
	}
	return value, nil
}
