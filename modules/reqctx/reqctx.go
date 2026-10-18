package reqctx

import (
	"context"
	"log"
	"strings"
)

// key is the type of the context keys, so they cannot collide with other packages' keys.
type key int

const (
	// SessionKey holds the signed-in session (*session.Session).
	SessionKey key = iota
	// RequestIDKey holds the request ID (int).
	RequestIDKey
	// RemoteAddrKey holds the client address (string).
	RemoteAddrKey
	// UserIDKey holds the signed-in user's ID (string).
	UserIDKey
)

const noUserID = "N/A"

// Log writes message prefixed with the request ID, user ID and remote address held in ctx.
func Log(ctx context.Context, message ...string) {
	remoteAddr := ctx.Value(RemoteAddrKey).(string)
	requestID := ctx.Value(RequestIDKey).(int)
	userID, ok := ctx.Value(UserIDKey).(string)
	if !ok {
		userID = noUserID
	}
	log.Printf("%d %s %s %s",
		requestID,
		userID,
		remoteAddr,
		strings.Join(message, " "),
	)
}
