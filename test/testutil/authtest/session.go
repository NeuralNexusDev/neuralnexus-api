package authtest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
)

// NewSessionRequest builds a request carrying session in its context and the given path values.
func NewSessionRequest(method string, session *session.Session, userID, platform, body string) *http.Request {
	ctx := context.WithValue(testutil.LogContext(context.Background()), mw.SessionKey, session)
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, "/", strings.NewReader(body)).WithContext(ctx)
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, "/", nil).WithContext(ctx)
	}
	if userID != "" {
		r.SetPathValue("user_id", userID)
	}
	if platform != "" {
		r.SetPathValue("platform", platform)
	}
	return r
}

// SelfSession returns a session for userID with no permissions.
func SelfSession(userID string) *session.Session {
	return &session.Session{UserID: userID}
}

// AdminUsersSession returns a session for userID holding the admin-users permission.
func AdminUsersSession(userID string) *session.Session {
	return &session.Session{UserID: userID, Permissions: []string{perms.ScopeAdminUsers.Node}}
}
