package auth

import "errors"

var (
	errBoom         = errors.New("boom")
	errCacheDown    = errors.New("cache down")
	errCacheMiss    = errors.New("miss")
	errDBDown       = errors.New("db down")
	errInsertFailed = errors.New("insert failed")
)
