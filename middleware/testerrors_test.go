package mw

import "errors"

var (
	errCacheDown = errors.New("cache down")
	errRedisDown = errors.New("redis down")
)
