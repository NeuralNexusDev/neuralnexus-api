package sess

import "github.com/google/uuid"

// SessionService interface
type SessionService interface {
	GetSession(id uuid.UUID) (*Session, error)
	UpdateSession(session *Session) (*Session, error)
	DeleteSession(id uuid.UUID) (*Session, error)
}

// sessionService - SessionService implementation
type sessionService struct {
	store SessionStore
}

// NewSessionService - Create a new session service
func NewSessionService(store SessionStore) SessionService {
	return &sessionService{
		store: store,
	}
}

// GetSession gets a session by ID
func (s *sessionService) GetSession(id uuid.UUID) (*Session, error) {
	session, err := s.store.GetSessionFromCache(id)
	if err != nil {
		session, err = s.store.GetSessionFromDB(id)
		if err != nil {
			return nil, err
		}
		s.store.AddSessionToCache(session)
	}
	return session, nil
}

// UpdateSession updates a session
func (s *sessionService) UpdateSession(session *Session) (*Session, error) {
	session, err := s.store.UpdateSessionInDB(session)
	if err != nil {
		return nil, err
	}
	if session.ID != uuid.Nil {
		s.store.AddSessionToCache(session)
	}
	return session, nil
}

// DeleteSession deletes a session by ID
func (s *sessionService) DeleteSession(id uuid.UUID) (*Session, error) {
	session, err := s.store.DeleteSessionInDB(id)
	if err != nil {
		return nil, err
	}
	if session.ID != uuid.Nil {
		s.store.DeleteSessionFromCache(id)
	}
	return session, nil
}
