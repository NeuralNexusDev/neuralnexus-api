package testutil

import "errors"

var (
	// ErrBodyRead stands in for a failure reading a request or response body.
	ErrBodyRead = errors.New("simulated body read failure")
	// ErrSigningFailed stands in for a failure signing a token.
	ErrSigningFailed = errors.New("signing failed")
	// ErrBoom is a generic failure returned by a dependency.
	ErrBoom = errors.New("boom")
	// ErrCacheDown stands in for the cache being unreachable.
	ErrCacheDown = errors.New("cache down")
	// ErrDBDown stands in for the database being unreachable.
	ErrDBDown = errors.New("db down")
	// ErrInsertFailed stands in for a failed insert.
	ErrInsertFailed = errors.New("insert failed")
	// ErrRedisDown stands in for Redis being unreachable.
	ErrRedisDown = errors.New("redis down")
	// ErrTransportFailed stands in for a failed HTTP round trip.
	ErrTransportFailed = errors.New("simulated transport failure")
)
