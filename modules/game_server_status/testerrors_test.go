package gss

import "errors"

var (
	errBodyRead           = errors.New("simulated body read failure")
	errServerUnreachable  = errors.New("server unreachable")
	errSimulatedTransport = errors.New("simulated transport failure")
)
