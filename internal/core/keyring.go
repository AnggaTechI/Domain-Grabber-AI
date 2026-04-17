package core

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Keyring manages a pool of API keys with automatic rotation on rate limits.
// Thread-safe. Designed for single-process use (in-memory state, resets per run).
type Keyring struct {
	mu       sync.Mutex
	keys     []string
	cooldown []time.Time // parallel array: when each key becomes usable again (zero = available)
	current  int         // current key index
	provider string      // for log/error context
}

// NewKeyring creates a keyring from a list of keys. Zero keys returns nil.
func NewKeyring(provider string, keys []string) *Keyring {
	if len(keys) == 0 {
		return nil
	}
	// Filter empty strings
	clean := make([]string, 0, len(keys))
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k != "" {
			clean = append(clean, k)
		}
	}
	if len(clean) == 0 {
		return nil
	}
	return &Keyring{
		keys:     clean,
		cooldown: make([]time.Time, len(clean)),
		current:  0,
		provider: provider,
	}
}

// Size returns the number of keys in the ring.
func (k *Keyring) Size() int {
	if k == nil {
		return 0
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	return len(k.keys)
}

// Current returns the currently active key and its 1-based index.
// Returns ("", 0) if all keys are in cooldown.
func (k *Keyring) Current() (key string, index int) {
	if k == nil {
		return "", 0
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	now := time.Now()
	// Check if current key is still valid
	if k.cooldown[k.current].IsZero() || now.After(k.cooldown[k.current]) {
		return k.keys[k.current], k.current + 1
	}
	// Current is in cooldown, find next available
	for offset := 1; offset < len(k.keys); offset++ {
		idx := (k.current + offset) % len(k.keys)
		if k.cooldown[idx].IsZero() || now.After(k.cooldown[idx]) {
			k.current = idx
			return k.keys[idx], idx + 1
		}
	}
	// All keys are in cooldown
	return "", 0
}

// MarkFailed puts the current key in cooldown for the given duration,
// rotates to the next available key, and returns info about what happened.
// If all keys are exhausted, returns allExhausted=true and the shortest
// wait time until any key becomes available again.
func (k *Keyring) MarkFailed(duration time.Duration) (nextIndex int, allExhausted bool, waitFor time.Duration) {
	if k == nil {
		return 0, true, 0
	}
	k.mu.Lock()
	defer k.mu.Unlock()

	failedIdx := k.current
	k.cooldown[failedIdx] = time.Now().Add(duration)

	now := time.Now()
	// Find next available key
	for offset := 1; offset <= len(k.keys); offset++ {
		idx := (failedIdx + offset) % len(k.keys)
		if idx == failedIdx {
			break
		}
		if k.cooldown[idx].IsZero() || now.After(k.cooldown[idx]) {
			k.current = idx
			return idx + 1, false, 0
		}
	}

	// All in cooldown - compute shortest wait
	var earliest time.Time
	for _, cd := range k.cooldown {
		if !cd.IsZero() && (earliest.IsZero() || cd.Before(earliest)) {
			earliest = cd
		}
	}
	if earliest.IsZero() {
		return 0, true, 0
	}
	wait := time.Until(earliest)
	if wait < 0 {
		wait = 0
	}
	return 0, true, wait
}

// AvailableCount returns how many keys are currently NOT in cooldown.
func (k *Keyring) AvailableCount() int {
	if k == nil {
		return 0
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	now := time.Now()
	count := 0
	for _, cd := range k.cooldown {
		if cd.IsZero() || now.After(cd) {
			count++
		}
	}
	return count
}

// Status returns a short human-readable summary of the keyring state.
func (k *Keyring) Status() string {
	if k == nil {
		return "no keys"
	}
	return fmt.Sprintf("%d/%d available", k.AvailableCount(), k.Size())
}

// IsRateLimitError returns true if the error message looks like a
// rate limit or quota error that should trigger key rotation.
// Covers common patterns across all supported providers.
func IsRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	patterns := []string{
		"429",
		"rate limit",
		"rate_limit",
		"ratelimit",
		"resource_exhausted",
		"resource exhausted",
		"quota",
		"too many requests",
		"credit balance",      // anthropic
		"insufficient_quota",  // openai
		"exceeded your current quota",
		"requests per minute",
	}
	for _, p := range patterns {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}

// IsTransientError returns true for errors that suggest retry (server overload)
// but NOT key rotation (same key should try again after a delay).
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	patterns := []string{
		"unavailable",
		"high demand",
		"overloaded",
		"503",
		"502",
		"504",
		"timeout",
		"connection reset",
	}
	for _, p := range patterns {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}
