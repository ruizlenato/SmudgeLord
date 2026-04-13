package utils

import (
	"math"
	"time"
)

func RetryWithBackoff[T any](
	fn func() (T, error),
	maxAttempts int,
	initialDelay time.Duration,
	maxDelay time.Duration,
	base float64,
) (T, error) {
	var result T
	var err error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		result, err = fn()
		if err == nil {
			return result, nil
		}

		if attempt < maxAttempts-1 {
			delay := time.Duration(math.Pow(base, float64(attempt))) * initialDelay
			if delay > maxDelay {
				delay = maxDelay
			}
			time.Sleep(delay)
		}
	}

	return result, err
}

func RetryWithBackoffSimple[T any](fn func() (T, error), maxAttempts int) (T, error) {
	return RetryWithBackoff(fn, maxAttempts, time.Second, 30*time.Second, 2)
}
