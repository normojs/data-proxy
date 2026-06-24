package service

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/model"
)

const tunnelRateLimitWindowSeconds = 60

var ErrTunnelRateLimited = errors.New("tunnel rate limit exceeded")

type TunnelRateLimitError struct {
	Metric        string
	Limit         int64
	Used          int64
	Attempted     int64
	WindowSeconds int64
}

func (err TunnelRateLimitError) Error() string {
	if err.Metric == "" {
		return ErrTunnelRateLimited.Error()
	}
	return fmt.Sprintf("%s: %s used=%d attempted=%d limit=%d window=%ds", ErrTunnelRateLimited.Error(), err.Metric, err.Used, err.Attempted, err.Limit, err.WindowSeconds)
}

func (err TunnelRateLimitError) Unwrap() error {
	return ErrTunnelRateLimited
}

func (err TunnelRateLimitError) Metadata() map[string]any {
	return map[string]any{
		"reason":         "rate_limited",
		"metric":         err.Metric,
		"limit":          err.Limit,
		"used":           err.Used,
		"attempted":      err.Attempted,
		"window_seconds": err.WindowSeconds,
	}
}

type tunnelRateLimitConfig struct {
	MaxRequestsPerMinute int64 `json:"max_requests_per_minute,omitempty"`
	MaxBytesInPerMinute  int64 `json:"max_bytes_in_per_minute,omitempty"`
	MaxBytesOutPerMinute int64 `json:"max_bytes_out_per_minute,omitempty"`
}

func (config tunnelRateLimitConfig) enabled() bool {
	return config.MaxRequestsPerMinute > 0 || config.MaxBytesInPerMinute > 0 || config.MaxBytesOutPerMinute > 0
}

type tunnelRateLimitBucket struct {
	windowStart time.Time
	requests    int64
	bytesIn     int64
	bytesOut    int64
}

type tunnelRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tunnelRateLimitBucket
	now     func() time.Time
}

var defaultTunnelRateLimiter = &tunnelRateLimiter{
	buckets: map[string]*tunnelRateLimitBucket{},
	now:     time.Now,
}

func checkTunnelRequestRateLimit(app model.TunnelApp, connection model.TunnelConnection, bytesIn int64) error {
	config := effectiveTunnelRateLimitConfig(app, connection)
	if !config.enabled() {
		return nil
	}
	return defaultTunnelRateLimiter.check(tunnelRateLimitKey(connection), config, true, bytesIn, 0)
}

func checkTunnelResponseRateLimit(app model.TunnelApp, connection model.TunnelConnection, bytesOut int64) error {
	config := effectiveTunnelRateLimitConfig(app, connection)
	if !config.enabled() {
		return nil
	}
	return defaultTunnelRateLimiter.check(tunnelRateLimitKey(connection), config, false, 0, bytesOut)
}

func resetTunnelRateLimiterForTest() {
	defaultTunnelRateLimiter.mu.Lock()
	defer defaultTunnelRateLimiter.mu.Unlock()
	defaultTunnelRateLimiter.buckets = map[string]*tunnelRateLimitBucket{}
	defaultTunnelRateLimiter.now = time.Now
}

func (limiter *tunnelRateLimiter) check(key string, config tunnelRateLimitConfig, countRequest bool, bytesIn int64, bytesOut int64) error {
	key = strings.TrimSpace(key)
	if key == "" || !config.enabled() {
		return nil
	}
	if bytesIn < 0 {
		bytesIn = 0
	}
	if bytesOut < 0 {
		bytesOut = 0
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	now := limiter.now()
	bucket := limiter.buckets[key]
	if bucket == nil || bucket.windowStart.IsZero() || now.Sub(bucket.windowStart) >= tunnelRateLimitWindowSeconds*time.Second || now.Before(bucket.windowStart) {
		bucket = &tunnelRateLimitBucket{windowStart: now}
		limiter.buckets[key] = bucket
	}

	if countRequest && config.MaxRequestsPerMinute > 0 && bucket.requests+1 > config.MaxRequestsPerMinute {
		return TunnelRateLimitError{
			Metric:        "requests_per_minute",
			Limit:         config.MaxRequestsPerMinute,
			Used:          bucket.requests,
			Attempted:     1,
			WindowSeconds: tunnelRateLimitWindowSeconds,
		}
	}
	if bytesIn > 0 && config.MaxBytesInPerMinute > 0 && bucket.bytesIn+bytesIn > config.MaxBytesInPerMinute {
		return TunnelRateLimitError{
			Metric:        "bytes_in_per_minute",
			Limit:         config.MaxBytesInPerMinute,
			Used:          bucket.bytesIn,
			Attempted:     bytesIn,
			WindowSeconds: tunnelRateLimitWindowSeconds,
		}
	}
	if bytesOut > 0 && config.MaxBytesOutPerMinute > 0 && bucket.bytesOut+bytesOut > config.MaxBytesOutPerMinute {
		return TunnelRateLimitError{
			Metric:        "bytes_out_per_minute",
			Limit:         config.MaxBytesOutPerMinute,
			Used:          bucket.bytesOut,
			Attempted:     bytesOut,
			WindowSeconds: tunnelRateLimitWindowSeconds,
		}
	}

	if countRequest {
		bucket.requests++
	}
	bucket.bytesIn += bytesIn
	bucket.bytesOut += bytesOut
	return nil
}

func tunnelRateLimitKey(connection model.TunnelConnection) string {
	if connection.Id > 0 {
		return fmt.Sprintf("connection:%d", connection.Id)
	}
	return "connection_key:" + connection.KeyHash
}

func effectiveTunnelRateLimitConfig(app model.TunnelApp, connection model.TunnelConnection) tunnelRateLimitConfig {
	appConfig := tunnelRateLimitConfigFromMap(unmarshalTunnelMap(app.BillingJson))
	connectionConfig := tunnelRateLimitConfigFromMap(unmarshalTunnelMap(connection.ConfigJson))
	return mergeTunnelRateLimitConfig(appConfig, connectionConfig)
}

func tunnelRateLimitConfigFromMap(values map[string]any) tunnelRateLimitConfig {
	if len(values) == 0 {
		return tunnelRateLimitConfig{}
	}
	if nested := mapFromAny(values["rate_limit"]); len(nested) > 0 {
		values = nested
	}
	return tunnelRateLimitConfig{
		MaxRequestsPerMinute: sanitizeTunnelRateLimitValue(int64FromAny(values["max_requests_per_minute"])),
		MaxBytesInPerMinute:  sanitizeTunnelRateLimitValue(int64FromAny(values["max_bytes_in_per_minute"])),
		MaxBytesOutPerMinute: sanitizeTunnelRateLimitValue(int64FromAny(values["max_bytes_out_per_minute"])),
	}
}

func mergeTunnelRateLimitConfig(appConfig tunnelRateLimitConfig, connectionConfig tunnelRateLimitConfig) tunnelRateLimitConfig {
	return tunnelRateLimitConfig{
		MaxRequestsPerMinute: tunnelMinPositive(appConfig.MaxRequestsPerMinute, connectionConfig.MaxRequestsPerMinute),
		MaxBytesInPerMinute:  tunnelMinPositive(appConfig.MaxBytesInPerMinute, connectionConfig.MaxBytesInPerMinute),
		MaxBytesOutPerMinute: tunnelMinPositive(appConfig.MaxBytesOutPerMinute, connectionConfig.MaxBytesOutPerMinute),
	}
}

func validateTunnelConnectionConfigJSON(raw string) error {
	config := tunnelRateLimitConfigFromMap(unmarshalTunnelMap(raw))
	if config.MaxRequestsPerMinute > 1000000 {
		return errors.New("max_requests_per_minute is too large")
	}
	if config.MaxBytesInPerMinute > 1<<40 {
		return errors.New("max_bytes_in_per_minute is too large")
	}
	if config.MaxBytesOutPerMinute > 1<<40 {
		return errors.New("max_bytes_out_per_minute is too large")
	}
	return nil
}

func sanitizeTunnelRateLimitValue(value int64) int64 {
	if value <= 0 {
		return 0
	}
	return value
}

func tunnelMinPositive(a int64, b int64) int64 {
	if a <= 0 {
		return b
	}
	if b <= 0 {
		return a
	}
	if a < b {
		return a
	}
	return b
}
