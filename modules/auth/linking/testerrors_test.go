package linking

import "errors"

var (
	errCleanupFailed    = errors.New("cleanup failed")
	errLookupFailed     = errors.New("lookup failed")
	errSessionStoreDown = errors.New("session store down")
	errWriteFailed      = errors.New("write failed")
)
