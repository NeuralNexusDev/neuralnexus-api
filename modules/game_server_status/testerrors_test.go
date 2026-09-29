package gss

import "errors"

var (
	errBodyRead           = errors.New("simulated body read failure")
	errSimulatedTransport = errors.New("simulated transport failure")
)
