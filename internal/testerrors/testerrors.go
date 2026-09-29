package testerrors

import "errors"

var (
	ErrBoom            = errors.New("boom")
	ErrCacheDown       = errors.New("cache down")
	ErrDBDown          = errors.New("db down")
	ErrInsertFailed    = errors.New("insert failed")
	ErrRedisDown       = errors.New("redis down")
	ErrTransportFailed = errors.New("simulated transport failure")
)
