package linking

import "errors"

var (
	errCleanupFailed    = errors.New("cleanup failed")
	errDBDown           = errors.New("db down")
	errInsertFailed     = errors.New("insert failed")
	errLinkFailed       = errors.New("link failed")
	errLinkInsertFailed = errors.New("link insert failed")
	errLookupFailed     = errors.New("lookup failed")
	errSessionStoreDown = errors.New("session store down")
	errWriteFailed      = errors.New("write failed")
)
