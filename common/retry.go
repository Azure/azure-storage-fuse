package common

import (
	"errors"
	"time"
)

// RetryWithTimeout retries `fn` using exponential backoff until `maxDuration` is exceeded.
// Backoff starts with `initialDelay` and caps at `maxDelay`.
func Retry[T any](maxDuration time.Duration, initialDelay time.Duration, maxDelay time.Duration, fn func() (T, error)) (T, error) {
	start := time.Now()
	if maxDuration <= 0 {
		maxDuration = 1 * time.Minute // Default to 1 minute if not specified
	}
	if initialDelay <= 0 {
		initialDelay = 1 * time.Millisecond // Default to 100ms if not specified
	}
	if maxDelay <= 0 {
		maxDelay = 10 * time.Second // Default to 10 seconds if not specified
	}
	delay := initialDelay
	var zero T
	var lastErr error

	for {
		result, err := fn()
		if err == nil {
			return result, nil
		}
		lastErr = err

		if time.Since(start)+delay > maxDuration {
			break
		}

		time.Sleep(delay)

		// Exponential backoff with max cap
		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}

	return zero, errors.New("retry timed out after " + maxDuration.String() + ": " + lastErr.Error())
}
