package twitch

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

// store struct for storing Twitch data
type store struct {
	db *pgxpool.Pool
}

// NewStore creates a new Twitch store
func NewStore(db *pgxpool.Pool) Store {
	return &store{db: db}
}

// CREATE DATABASE twitch;

//CREATE TABLE eventsub_subscriptions (
//  id UUID PRIMARY KEY,
//  user_id TEXT NOT NULL,
//  status TEXT,
//  type TEXT,
//  version INTEGER,
//  created_at TIMESTAMPTZ,
//  revoked_at TIMESTAMPTZ,
//  cost INTEGER
//);

// EventSubEntry struct for storing EventSub subscription data
type EventSubEntry struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Status    string    `json:"status"`
	Type      string    `json:"type"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	RevokedAt time.Time `json:"revoked_at"`
	Cost      int       `json:"cost"`
}

// Store struct for storing Twitch data
type Store interface {
	GetEventSubSubscription(id string) (*EventSubEntry, error)
	GetEventSubSubscriptions(userID string) ([]*EventSubEntry, error)
	CreateEventSubSubscription(entry *EventSubEntry) error
	UpdateEventSubSubscription(entry *EventSubEntry) error
}

// GetEventSubSubscription gets an EventSub subscription by ID
func (s *store) GetEventSubSubscription(id string) (*EventSubEntry, error) {
	rows, err := s.db.Query(context.Background(), "SELECT * FROM eventsub_subscriptions WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entry, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[EventSubEntry])
	if err != nil {
		return nil, err
	}
	return entry, nil
}

// GetEventSubSubscriptions gets all EventSub subscriptions for a user
func (s *store) GetEventSubSubscriptions(userID string) ([]*EventSubEntry, error) {
	rows, err := s.db.Query(context.Background(), "SELECT * FROM eventsub_subscriptions WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[EventSubEntry])
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// CreateEventSubSubscription creates a new EventSub subscription
func (s *store) CreateEventSubSubscription(entry *EventSubEntry) error {
	_, err := s.db.Exec(context.Background(), "INSERT INTO eventsub_subscriptions (id, user_id, status, type, version, created_at, revoked_at, cost) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		entry.ID, entry.UserID, entry.Status, entry.Type, entry.Version, entry.CreatedAt, entry.RevokedAt, entry.Cost)
	if err != nil {
		return err
	}
	return nil
}

// UpdateEventSubSubscription updates an EventSub subscription
func (s *store) UpdateEventSubSubscription(entry *EventSubEntry) error {
	_, err := s.db.Exec(context.Background(), "UPDATE eventsub_subscriptions SET status = $1, type = $2, version = $3, created_at = $4, revoked_at = $5, cost = $6 WHERE id = $7",
		entry.Status, entry.Type, entry.Version, entry.CreatedAt, entry.RevokedAt, entry.Cost, entry.ID)
	if err != nil {
		return err
	}
	return nil
}

// EventSubService interface for handling EventSub subscriptions
type EventSubService interface {
	GetEventSubSubscription(id string) (*EventSubEntry, error)
	GetEventSubSubscriptions(userID string) ([]*EventSubEntry, error)
	CreateEventSubSubscription(entry *EventSubEntry) error
	UpdateEventSubSubscriptionStatus(id string, status string) error
	RevokeEventSubSubscription(id string, status string) error
}

// service struct for handling EventSub subscriptions
type service struct {
	store Store
}

// NewService creates a new EventSub service
func NewService(store Store) EventSubService {
	return &service{store: store}
}

// GetEventSubSubscription gets an EventSub subscription by ID
func (s *service) GetEventSubSubscription(id string) (*EventSubEntry, error) {
	return s.store.GetEventSubSubscription(id)
}

// GetEventSubSubscriptions gets all EventSub subscriptions for a user
func (s *service) GetEventSubSubscriptions(userID string) ([]*EventSubEntry, error) {
	return s.store.GetEventSubSubscriptions(userID)
}

// CreateEventSubSubscription creates a new EventSub subscription
func (s *service) CreateEventSubSubscription(entry *EventSubEntry) error {
	return s.store.CreateEventSubSubscription(entry)
}

// UpdateEventSubSubscriptionStatus updates an EventSub subscription's status
func (s *service) UpdateEventSubSubscriptionStatus(id string, status string) error {
	entry, err := s.store.GetEventSubSubscription(id)
	if err != nil {
		return err
	}
	entry.Status = status
	return s.store.UpdateEventSubSubscription(entry)
}

// RevokeEventSubSubscription revokes an EventSub subscription
func (s *service) RevokeEventSubSubscription(id string, status string) error {
	entry, err := s.store.GetEventSubSubscription(id)
	if err != nil {
		return err
	}
	entry.Status = status
	entry.RevokedAt = time.Now()
	return s.store.UpdateEventSubSubscription(entry)
}
