package auth

import "errors"

var (
	errBoom         = errors.New("boom")
	errCacheDown    = errors.New("cache down")
	errDBDown       = errors.New("db down")
	errInsertFailed = errors.New("insert failed")
)
