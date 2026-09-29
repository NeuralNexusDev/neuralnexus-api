package authroutes

import "errors"

var (
	errDBExploded    = errors.New("db exploded")
	errSigningFailed = errors.New("signing failed")
)
