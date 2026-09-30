package testerrors

import "errors"

var (
	ErrBodyRead        = errors.New("simulated body read failure")
	ErrSigningFailed   = errors.New("signing failed")
	ErrBoom            = errors.New("boom")
	ErrCacheDown       = errors.New("cache down")
	ErrDBDown          = errors.New("db down")
	ErrInsertFailed    = errors.New("insert failed")
	ErrRedisDown       = errors.New("redis down")
	ErrTransportFailed = errors.New("simulated transport failure")
)
