package gss

import "errors"

var (
	errServerUnreachable  = errors.New("server unreachable")
	errSimulatedTransport = errors.New("simulated transport failure")
)
